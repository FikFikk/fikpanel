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

func TestCollector(t *testing.T) {
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
}

