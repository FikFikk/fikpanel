package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateBlowfishSecret(t *testing.T) {
	secret1 := GenerateBlowfishSecret()
	secret2 := GenerateBlowfishSecret()

	if len(secret1) != 32 {
		t.Errorf("diharapkan panjang blowfish secret 32 karakter, didapat %d (%s)", len(secret1), secret1)
	}

	if secret1 == secret2 {
		t.Errorf("blowfish secret harus acak dan unik untuk tiap panggilan")
	}
}

func TestEnsureConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel-pma-test-*")
	if err != nil {
		t.Fatalf("gagal membuat direktori temporer pengujian: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ensureConfigFile(tempDir)

	cfgPath := filepath.Join(tempDir, "config.inc.php")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("berkas config.inc.php seharusnya terbentuk di %s: %v", cfgPath, err)
	}

	content := string(data)
	if !strings.Contains(content, "$cfg['blowfish_secret']") {
		t.Errorf("config.inc.php harus memuat deklarasi blowfish_secret")
	}
	if !strings.Contains(content, "$cfg['Servers'][$i]['auth_type'] = 'cookie'") {
		t.Errorf("config.inc.php harus memuat autentikasi berbasis cookie")
	}
	if !strings.Contains(content, "$cfg['Servers'][$i]['port'] = '3306'") {
		t.Errorf("config.inc.php harus mengonfigurasi port 3306")
	}
}

func TestGetPhpMyAdminTargetDir(t *testing.T) {
	dir := GetPhpMyAdminTargetDir()
	if dir == "" {
		t.Errorf("GetPhpMyAdminTargetDir tidak boleh mengembalikan string kosong")
	}
}
