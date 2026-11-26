package docker

import (
	"strings"
	"testing"
)

func TestParseHumanSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0B", 0},
		{"512B", 512},
		{"10kB", 10000},
		{"1KiB", 1024},
		{"142MB", 142000000},
		{"1.5GiB", 1610612736},
		{"256MiB", 268435456},
		{"12.5MB (virtual 180MB)", 12500000},
		{"N/A", 0},
	}
	for _, tc := range cases {
		got := ParseHumanSize(tc.in)
		if got != tc.want {
			t.Errorf("ParseHumanSize(%q) = %d, ingin %d", tc.in, got, tc.want)
		}
	}
}

func TestParseDockerPorts(t *testing.T) {
	raw := "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp, 0.0.0.0:8443->443/tcp, 3306/tcp"
	ports := ParseDockerPorts(raw)
	if len(ports) != 3 {
		t.Fatalf("diharapkan 3 port unik pasca deduplikasi IPv4/IPv6, dapat %d: %+v", len(ports), ports)
	}
	if ports[0].PublicPort != 8080 || ports[0].PrivatePort != 80 || ports[0].Type != "tcp" {
		t.Errorf("port pertama tidak sesuai: %+v", ports[0])
	}
	if ports[1].PublicPort != 8443 || ports[1].PrivatePort != 443 {
		t.Errorf("port kedua tidak sesuai: %+v", ports[1])
	}
	if ports[2].PublicPort != 0 || ports[2].PrivatePort != 3306 {
		t.Errorf("port ekspos internal tidak sesuai: %+v", ports[2])
	}
}

func TestParseSystemDFOutput(t *testing.T) {
	sample := `{"Type":"Images","TotalCount":"8","Active":"3","Size":"1.2GB","Reclaimable":"450MB (37%)"}
{"Type":"Containers","TotalCount":"4","Active":"2","Size":"32MB","Reclaimable":"12MB (37%)"}
{"Type":"Local Volumes","TotalCount":"5","Active":"2","Size":"500MB","Reclaimable":"200MB (40%)"}
{"Type":"Build Cache","TotalCount":"12","Active":"0","Size":"150MB","Reclaimable":"150MB (100%)"}`

	df := ParseSystemDFOutput(sample)
	if df.Images.TotalCount != 8 || df.Images.ActiveCount != 3 || df.Images.SizeBytes != 1200000000 {
		t.Errorf("ringkasan Images salah: %+v", df.Images)
	}
	if df.BuildCache.ReclaimablePct != 100 {
		t.Errorf("reclaimable pct build cache salah: %v", df.BuildCache.ReclaimablePct)
	}
	expectedTotal := int64(1200000000 + 32000000 + 500000000 + 150000000)
	if df.TotalSizeBytes != expectedTotal {
		t.Errorf("total size bytes = %d, ingin %d", df.TotalSizeBytes, expectedTotal)
	}
}

func TestParseComposeStatusSummary(t *testing.T) {
	st, total, run := ParseComposeStatusSummary("running(3), exited(1)")
	if st != "partial" || total != 4 || run != 3 {
		t.Errorf("status partial salah: %s %d %d", st, total, run)
	}

	st2, total2, run2 := ParseComposeStatusSummary("running(2)")
	if st2 != "running" || total2 != 2 || run2 != 2 {
		t.Errorf("status running salah: %s %d %d", st2, total2, run2)
	}

	st3, total3, run3 := ParseComposeStatusSummary("exited(2)")
	if st3 != "stopped" || total3 != 2 || run3 != 0 {
		t.Errorf("status stopped salah: %s %d %d", st3, total3, run3)
	}
}

func TestValidationHelpers(t *testing.T) {
	if !IsValidDockerRef("nginx:1.27-alpine") {
		t.Error("nginx:1.27-alpine seharusnya valid")
	}
	if !IsValidDockerRef("ghcr.io/user/app:v1.0.0") {
		t.Error("ghcr.io/user/app:v1.0.0 seharusnya valid")
	}
	if IsValidDockerRef("--privileged") || IsValidDockerRef("nginx;rm -rf /") {
		t.Error("input injeksi seharusnya ditolak")
	}
	if !IsValidProjectName("my-stack_01") {
		t.Error("my-stack_01 seharusnya valid")
	}
	if IsValidProjectName("../escape") || IsValidProjectName("bad space") {
		t.Error("nama proyek berbahaya seharusnya ditolak")
	}
	if !IsSecretEnvKey("POSTGRES_PASSWORD") || !IsSecretEnvKey("JWT_SECRET") || IsSecretEnvKey("NODE_ENV") {
		t.Error("deteksi variabel rahasia tidak akurat")
	}
}

func TestRenderTemplateComposeOverrides(t *testing.T) {
	mgr := GetManager()
	templates := mgr.GetTemplates()
	if len(templates) < 15 {
		t.Fatalf("diharapkan minimal 15 template One-Click, ditemukan %d", len(templates))
	}

	var wp OneClickTemplate
	for _, tpl := range templates {
		if tpl.ID == "wordpress" {
			wp = tpl
			break
		}
	}
	if wp.ID == "" {
		t.Fatal("template wordpress tidak ditemukan")
	}

	rendered := RenderTemplateCompose(wp, DeployTemplateRequest{
		TemplateID: "wordpress",
		Name:       "blog-utama",
		Port:       9090,
		VolumePath: "/www/wp_blog",
		EnvValues: map[string]string{
			"WORDPRESS_DB_PASSWORD": "SuperSecretPassword99!",
		},
	})

	if !strings.Contains(rendered, `"9090:80"`) {
		t.Errorf("override port 9090:80 tidak diterapkan:\n%s", rendered)
	}
	if !strings.Contains(rendered, "/www/wp_blog:/var/www/html") {
		t.Errorf("override volume path tidak diterapkan:\n%s", rendered)
	}
	if !strings.Contains(rendered, "WORDPRESS_DB_PASSWORD: SuperSecretPassword99!") {
		t.Errorf("override env WORDPRESS_DB_PASSWORD tidak diterapkan:\n%s", rendered)
	}
	if !strings.Contains(rendered, "MYSQL_PASSWORD: SuperSecretPassword99!") {
		t.Errorf("sinkronisasi env MYSQL_PASSWORD pada WordPress tidak diterapkan:\n%s", rendered)
	}
}

func TestStreamTaskRegistryLifecycle(t *testing.T) {
	reg := newStreamTaskRegistry()
	task := reg.Create("pull", "nginx:alpine")
	task.Append("Pulling from library/nginx")

	snap, ch, cleanup, ok := reg.Subscribe(task.id)
	if !ok || len(snap.Logs) != 1 || ch == nil {
		t.Fatalf("subscribe awal gagal: ok=%v logs=%d", ok, len(snap.Logs))
	}
	defer cleanup()

	task.Append("Download complete")
	line := <-ch
	if line != "Download complete" {
		t.Errorf("baris stream salah: %q", line)
	}

	task.Finish(nil)
	if _, open := <-ch; open {
		t.Error("kanal stream seharusnya ditutup setelah Finish")
	}
	finalSnap, found := reg.GetSnapshot(task.id)
	if !found || finalSnap.Status != "success" {
		t.Errorf("status akhir seharusnya success, dapat %+v", finalSnap)
	}
}
