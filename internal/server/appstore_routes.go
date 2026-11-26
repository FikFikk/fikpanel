package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// writeAppStoreErr menulis respons galat JSON ter-escape dengan benar untuk seluruh endpoint App Store
func writeAppStoreErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    false,
		"error": msg,
	})
}

func (s *Server) handleListAppStore(w http.ResponseWriter, r *http.Request) {
	forceRefresh := r.URL.Query().Get("refresh") == "1" || strings.EqualFold(r.URL.Query().Get("refresh"), "true")
	catalog := s.appStore.CatalogWithRefresh(r.Context(), forceRefresh)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"apps": catalog,
	})
}

func (s *Server) handleListAppBundles(w http.ResponseWriter, r *http.Request) {
	bundles := s.appStore.Bundles(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"bundles": bundles,
	})
}

func (s *Server) handleInstallAppBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := strings.TrimSpace(r.PathValue("id"))
	if bundleID == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "ID stack bundle wajib diisi")
		return
	}

	taskID, err := s.appStore.InstallBundle(r.Context(), bundleID)
	if err != nil {
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.bundle_install", bundleID, true, "Memicu instalasi Stack Bundle "+bundleID, nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": taskID,
		"message": "Instalasi Stack Bundle telah dijadwalkan di background",
	})
}

func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(r.PathValue("id"))
	if appID == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "ID aplikasi wajib diisi")
		return
	}

	if err := s.appStore.Install(appID); err != nil {
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.install", appID, true, "Memicu 1-Click instalasi "+appID, nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": appID,
		"message": "Instalasi telah dimulai di background",
	})
}

func (s *Server) handleUpgradeApp(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(r.PathValue("id"))
	if appID == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "ID aplikasi wajib diisi")
		return
	}

	if err := s.appStore.Upgrade(appID); err != nil {
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.upgrade", appID, true, "Memicu upgrade paket "+appID, nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": "upgrade:" + appID,
		"message": "Pembaruan paket telah dijadwalkan di background",
	})
}

func (s *Server) handleUninstallApp(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(r.PathValue("id"))
	if appID == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "ID aplikasi wajib diisi")
		return
	}

	if err := s.appStore.Uninstall(appID); err != nil {
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.uninstall", appID, true, "Memicu pencopotan "+appID, nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": appID,
		"message": "Pencopotan paket telah dimulai di background",
	})
}

func (s *Server) handleAptUpdate(w http.ResponseWriter, r *http.Request) {
	if err := s.appStore.AptUpdate(); err != nil {
		writeAppStoreErr(w, http.StatusConflict, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.apt_update", "apt", true, "Menyinkronkan indeks repositori APT", nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": "sys:apt-update",
		"message": "Sinkronisasi indeks repositori APT dimulai",
	})
}

func (s *Server) handleControlAppService(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(r.PathValue("id"))
	action := strings.TrimSpace(r.PathValue("action"))
	if appID == "" || action == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "ID aplikasi dan aksi layanan wajib diisi")
		return
	}

	msg, err := s.appStore.ControlService(r.Context(), appID, action)
	if err != nil {
		_ = s.store.RecordAudit(r.Context(), "user", "appstore.service_"+action, appID, false, err.Error(), nil)
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.service_"+action, appID, true, msg, nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": msg,
	})
}

func (s *Server) handleInspectAppPackage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeAppStoreErr(w, http.StatusBadRequest, "nama paket wajib diisi")
		return
	}

	detail, err := s.appStore.InspectPackage(r.Context(), name)
	if err != nil {
		writeAppStoreErr(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"detail": detail,
	})
}

func (s *Server) handleListAppUpgradable(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("refresh") == "1"
	items := s.appStore.ListUpgradable(r.Context(), force)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"packages": items,
	})
}

func (s *Server) handleListAppTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.appStore.ListTasks()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"tasks": tasks,
	})
}

func (s *Server) handleGetAppTask(w http.ResponseWriter, r *http.Request) {
	appID := strings.TrimSpace(r.PathValue("id"))
	task := s.appStore.GetTask(appID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"task": task,
	})
}

func (s *Server) handleSearchAppStore(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pkgs, err := s.appStore.SearchGlobalPackages(r.Context(), q)
	if err != nil {
		writeAppStoreErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"packages": pkgs,
	})
}
