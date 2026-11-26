package appstore

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestIsValidDebianPackageAndVersionCleaning(t *testing.T) {
	valid := []string{"nginx", "php8.4-fpm", "libssl3t64", "g++", "python3-pip", "mariadb-server"}
	for _, p := range valid {
		if !IsValidDebianPackage(p) {
			t.Errorf("ekspektasi paket %q valid", p)
		}
	}
	invalid := []string{"", "--force-yes", "-y", "nginx; rm -rf /", "Nginx", "../etc/passwd", "pkg name"}
	for _, p := range invalid {
		if IsValidDebianPackage(p) {
			t.Errorf("ekspektasi paket berbahaya %q ditolak", p)
		}
	}

	cases := map[string]string{
		"1:10.11.8-0+deb12u1": "10.11.8",
		"1.24.0-2ubuntu7.1":   "1.24.0",
		"2:8.3.14+ubuntu24.04": "8.3.14",
		"26.1.4~dfsg1-1":      "26.1.4",
	}
	for raw, want := range cases {
		if got := cleanDebianVersion(raw); got != want {
			t.Errorf("cleanDebianVersion(%q) = %q, ingin %q", raw, got, want)
		}
	}
}

func TestParsersDpkgUpgradableAndAptShow(t *testing.T) {
	// 1. Parser output batch dpkg-query
	dpkgRaw := "nginx\tinstall ok installed\t1.24.0-2ubuntu7\t1840\n" +
		"apache2\tdeinstall ok config-files\t2.4.58-1\t500\n" +
		"redis-server\tinstall ok installed\t5:7.0.15-1build2\t1320\n"
	parsedDpkg := parseBatchDpkgOutput(dpkgRaw)
	if !parsedDpkg["nginx"].installed || parsedDpkg["nginx"].sizeKB != 1840 {
		t.Fatalf("nginx harus terdeteksi terpasang dengan size 1840 KB, dapat %+v", parsedDpkg["nginx"])
	}
	if parsedDpkg["apache2"].installed {
		t.Fatalf("apache2 berstatus config-files tidak boleh dianggap terpasang")
	}
	if !parsedDpkg["redis-server"].installed || cleanDebianVersion(parsedDpkg["redis-server"].version) != "7.0.15" {
		t.Fatalf("redis-server harus terdeteksi 7.0.15, dapat %+v", parsedDpkg["redis-server"])
	}

	// 2. Parser output apt list --upgradable
	aptUpRaw := "Listing...\n" +
		"nginx/noble-updates 1.26.2-1ubuntu1 arm64 [upgradable from: 1.24.0-2ubuntu7]\n" +
		"docker.io/noble-security 27.3.1-0ubuntu1 arm64 [upgradable from: 26.1.4-0ubuntu1]\n"
	upMap := ParseAptUpgradableOutput(aptUpRaw)
	if len(upMap) != 2 || upMap["nginx"].CandidateVersion != "1.26.2-1ubuntu1" {
		t.Fatalf("hasil parse upgradable tidak sesuai: %+v", upMap)
	}

	// 3. Parser output apt-cache show
	showRaw := "Package: nginx\n" +
		"Version: 1.26.2-1ubuntu1\n" +
		"Section: web\n" +
		"Maintainer: Ubuntu Developers <ubuntu@ubuntu.com>\n" +
		"Installed-Size: 1920\n" +
		"Size: 524288\n" +
		"Depends: libc6, libssl3\n" +
		"Homepage: https://nginx.org\n" +
		"Description-en: small, powerful, scalable web/proxy server\n" +
		" Nginx is a high-performance HTTP server.\n" +
		" .\n" +
		" Supports reverse proxy and SSL.\n\n" +
		"Package: nginx\nVersion: 1.24.0-2ubuntu7\n"
	var det PackageDetail
	parseAptCacheShow(showRaw, &det)
	if det.CandidateVersion != "1.26.2" || det.Section != "web" || det.DownloadSize != "512 KB" {
		t.Fatalf("parseAptCacheShow tidak sesuai: %+v", det)
	}
	if !strings.Contains(det.Description, "Supports reverse proxy") {
		t.Fatalf("deskripsi multi-baris tidak ter-parse: %q", det.Description)
	}
}

func TestManagerCatalogBundlesTasksAndSecurity(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	// Pastikan katalog modern memiliki >= 25 paket dan ikon Fail2Ban benar (huruf kecil)
	cat := mgr.Catalog(ctx)
	if len(cat) < 25 {
		t.Fatalf("ekspektasi minimal 25 paket katalog, dapat %d", len(cat))
	}
	for _, a := range cat {
		if a.ID == "fail2ban" && a.Icon != "ri-alarm-warning-line" {
			t.Fatalf("ikon fail2ban harus huruf kecil ri-alarm-warning-line, dapat %q", a.Icon)
		}
		if a.ID == "php8.4" && a.ManagerRoute != "#/php" {
			t.Fatalf("php8.4 harus punya ManagerRoute #/php, dapat %q", a.ManagerRoute)
		}
		if a.ID == "certbot" && a.ManagerRoute == "#dns" {
			t.Fatalf("certbot tidak boleh mengarah ke rute #dns yang tidak valid")
		}
	}

	// Tolak injeksi flag pada pkg:
	if err := mgr.Install("pkg:--assume-yes"); err == nil {
		t.Fatal("ekspektasi error saat menginstal nama paket dengan flag --")
	}
	if err := mgr.Uninstall("pkg:nginx;id"); err == nil {
		t.Fatal("ekspektasi error saat mencopot paket dengan karakter shell")
	}

	// Cek bundel stack 1-klik
	bundles := mgr.Bundles(ctx)
	if len(bundles) < 5 {
		t.Fatalf("ekspektasi minimal 5 stack bundle, dapat %d", len(bundles))
	}

	// Cek AptUpdate & daftar task
	if err := mgr.AptUpdate(); err != nil {
		t.Fatalf("AptUpdate gagal: %v", err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		tk := mgr.GetTask("sys:apt-update")
		if tk != nil && tk.Status == "success" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	tk := mgr.GetTask("sys:apt-update")
	if tk == nil || tk.Status != "success" {
		t.Fatalf("ekspektasi task sys:apt-update sukses, dapat %+v", tk)
	}
	if len(mgr.ListTasks()) == 0 {
		t.Fatal("ListTasks harus memuat task yang baru dijalankan")
	}

	// Uji validasi aksi kontrol layanan
	if _, err := mgr.ControlService(ctx, "nginx", "destroy"); err == nil {
		t.Fatal("aksi layanan tidak dikenal harus ditolak")
	}
	if _, err := mgr.ControlService(ctx, "git", "restart"); err == nil {
		t.Fatal("paket non-daemon git harus ditolak oleh ControlService")
	}
	if msg, err := mgr.ControlService(ctx, "nginx", "restart"); err != nil || !strings.Contains(msg, "nginx") {
		t.Fatalf("ControlService nginx restart gagal: msg=%q err=%v", msg, err)
	}

	// Uji InspectPackage
	det, err := mgr.InspectPackage(ctx, "nginx")
	if err != nil || det == nil || det.Name != "nginx" {
		t.Fatalf("InspectPackage nginx gagal: det=%+v err=%v", det, err)
	}
}
