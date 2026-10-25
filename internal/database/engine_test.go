package database

import (
	"context"
	"testing"
)

func TestDetectEngines(t *testing.T) {
	ctx := context.Background()
	counts := map[string]int{
		"mysql": 2,
	}

	engines := DetectEngines(ctx, counts)
	if len(engines) != 4 {
		t.Fatalf("diharapkan 4 engine, didapat %d", len(engines))
	}

	var mongo EngineInfo
	var mysql EngineInfo
	for _, e := range engines {
		if e.Name == "mongodb" {
			mongo = e
		}
		if e.Name == "mysql" {
			mysql = e
		}
	}

	if mysql.DatabasesCount != 2 {
		t.Errorf("diharapkan jumlah db mysql = 2, didapat %d", mysql.DatabasesCount)
	}

	// Sebelum diinstal, mongodb harus berstatus false (sesuai referensi tangkapan layar aaPanel)
	if mongo.Installed {
		t.Errorf("mongodb awalnya harus belum terpasang (installed=false)")
	}

	// Uji simulasi perubahan status instalasi
	SetEngineInstalled("mongodb", "7.0.5", true)
	updatedEngines := DetectEngines(ctx, counts)
	for _, e := range updatedEngines {
		if e.Name == "mongodb" {
			if !e.Installed || e.Version != "7.0.5" || !e.Running {
				t.Errorf("mongodb seharusnya sudah berstatus installed=true, version=7.0.5, didapat %+v", e)
			}
		}
	}
}
