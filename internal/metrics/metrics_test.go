package metrics

import (
	"testing"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push(Point{Timestamp: 1})
	rb.Push(Point{Timestamp: 2})

	pts := rb.GetAll()
	if len(pts) != 2 {
		t.Fatalf("Harus terdapat 2 data dalam buffer, ditemukan: %d", len(pts))
	}
	if pts[0].Timestamp != 1 || pts[1].Timestamp != 2 {
		t.Errorf("Urutan data metrik tidak sesuai: %+v", pts)
	}

	// Uji perilaku ring buffer saat kapasitas penuh (overflow)
	rb.Push(Point{Timestamp: 3})
	rb.Push(Point{Timestamp: 4})

	ptsFull := rb.GetAll()
	if len(ptsFull) != 3 {
		t.Fatalf("Harus terdapat 3 data maksimal saat penuh, ditemukan: %d", len(ptsFull))
	}
	if ptsFull[0].Timestamp != 2 || ptsFull[1].Timestamp != 3 || ptsFull[2].Timestamp != 4 {
		t.Errorf("Rotasi data ring buffer salah: %+v", ptsFull)
	}
}

func TestCollectorAndHubCollectFresh(t *testing.T) {
	col := NewCollector()
	pt, err := col.Collect()
	if err != nil {
		t.Fatalf("Gagal mengumpulkan data metrik: %v", err)
	}
	if pt.Timestamp == 0 {
		t.Error("Timestamp metrik tidak boleh 0")
	}
	if pt.RAMTotal == 0 {
		t.Error("RAM total sistem tidak boleh 0")
	}

	// Pastikan CollectFresh selalu menghasilkan sampel terbaru meski tanpa subscriber SSE
	hub := NewHub()
	fresh := hub.CollectFresh()
	if fresh.Timestamp == 0 || fresh.RAMTotal == 0 {
		t.Fatalf("CollectFresh tidak mengembalikan metrik valid: %+v", fresh)
	}
}

// TestParseProcStatAndCPUDelta memverifikasi parsing /proc/stat agregat + per-core serta pecahan User/System/IOWait/Steal
func TestParseProcStatAndCPUDelta(t *testing.T) {
	raw1 := `cpu  100 0 50 800 30 0 10 10 0 0
cpu0 50 0 25 400 15 0 5 5 0 0
cpu1 50 0 25 400 15 0 5 5 0 0
intr 12345
`
	raw2 := `cpu  140 0 60 830 40 0 10 20 0 0
cpu0 80 0 30 410 20 0 5 10 0 0
cpu1 60 0 30 420 20 0 5 10 0 0
`
	agg1, cores1 := parseProcStatRaw(raw1)
	agg2, cores2 := parseProcStatRaw(raw2)

	if len(cores1) != 2 || len(cores2) != 2 {
		t.Fatalf("diharapkan 2 core CPU, dapat %d dan %d", len(cores1), len(cores2))
	}
	// Delta total = 40+10+30+10+10 = 100 tick; idle+iowait = 30+10 = 40 -> total busy = 60.0%
	bd := computeCPUDelta(agg1, agg2)
	if bd.Total != 60.0 || bd.User != 40.0 || bd.System != 10.0 || bd.IOWait != 10.0 || bd.Steal != 10.0 {
		t.Fatalf("pecahan CPU tidak akurat: %+v", bd)
	}
}

// TestParseDiskStats memverifikasi ekstraksi byte baca/tulis hanya dari perangkat blok fisik (tanpa partisi/loop)
func TestParseDiskStats(t *testing.T) {
	raw := `   7       0 loop0 100 0 2000 0 0 0 0 0 0 0 0
   8       0 sda 100 0 1000 0 50 0 2000 0 0 0 0
   8       1 sda1 100 0 1000 0 50 0 2000 0 0 0 0
 259       0 nvme0n1 10 0 400 0 20 0 600 0 0 0 0
 259       1 nvme0n1p1 10 0 400 0 20 0 600 0 0 0 0
`
	rd, wr := parseDiskStatsRaw(raw)
	wantRd := uint64(1000+400) * 512
	wantWr := uint64(2000+600) * 512
	if rd != wantRd || wr != wantWr {
		t.Fatalf("diskstats tidak sesuai (partisi/loop ikut terhitung): got rd=%d wr=%d, want rd=%d wr=%d", rd, wr, wantRd, wantWr)
	}
}
