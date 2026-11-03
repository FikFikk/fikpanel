package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectRuntime_PHP_Laravel(t *testing.T) {
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("Gagal membuat folder public: %v", err)
	}

	// Buat file artisan di parent direktori
	artisanFile := filepath.Join(tmpDir, "artisan")
	if err := os.WriteFile(artisanFile, []byte("#!/usr/bin/env php\n<?php"), 0o755); err != nil {
		t.Fatalf("Gagal membuat file artisan: %v", err)
	}

	info := DetectProjectRuntime(publicDir, "", "kostash.my.id")
	if info.Kind != "php" {
		t.Errorf("Kind salah: got %s, want php", info.Kind)
	}
	if info.Framework != "Laravel" {
		t.Errorf("Framework salah: got %s, want Laravel", info.Framework)
	}
}

func TestDetectProjectRuntime_Node_Hono_And_Nuxt(t *testing.T) {
	// 1. Test Hono
	tmpHono := t.TempDir()
	pkgHono := `{"dependencies": {"@hono/node-server": "^1.0.0", "hono": "^4.0.0"}}`
	_ = os.WriteFile(filepath.Join(tmpHono, "package.json"), []byte(pkgHono), 0o644)

	infoHono := DetectProjectRuntime(tmpHono, "", "hono-app.test")
	if infoHono.Kind != "node" || infoHono.Framework != "Hono" {
		t.Errorf("Hono deteksi salah: got %s/%s, want node/Hono", infoHono.Kind, infoHono.Framework)
	}

	// 2. Test Nuxt
	tmpNuxt := t.TempDir()
	pkgNuxt := `{"devDependencies": {"nuxt": "^3.10.0"}}`
	_ = os.WriteFile(filepath.Join(tmpNuxt, "package.json"), []byte(pkgNuxt), 0o644)

	infoNuxt := DetectProjectRuntime(tmpNuxt, "", "nuxt-app.test")
	if infoNuxt.Kind != "node" || infoNuxt.Framework != "Nuxt" {
		t.Errorf("Nuxt deteksi salah: got %s/%s, want node/Nuxt", infoNuxt.Kind, infoNuxt.Framework)
	}
}

func TestDetectProjectRuntime_Go_And_Python(t *testing.T) {
	// 1. Test Go
	tmpGo := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpGo, "go.mod"), []byte("module myapp\ngo 1.22"), 0o644)

	infoGo := DetectProjectRuntime(tmpGo, "", "go-api.test")
	if infoGo.Kind != "go" || infoGo.Framework != "Go Service" {
		t.Errorf("Go deteksi salah: got %s/%s, want go/Go Service", infoGo.Kind, infoGo.Framework)
	}

	// 2. Test Python FastAPI
	tmpPy := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpPy, "requirements.txt"), []byte("fastapi==0.110.0\nuvicorn\n"), 0o644)

	infoPy := DetectProjectRuntime(tmpPy, "", "py-api.test")
	if infoPy.Kind != "python" || infoPy.Framework != "FastAPI" {
		t.Errorf("Python deteksi salah: got %s/%s, want python/FastAPI", infoPy.Kind, infoPy.Framework)
	}
}
