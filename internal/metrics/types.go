package metrics

// Point merepresentasikan satu titik metrik sistem pada waktu tertentu
type Point struct {
	Timestamp int64   `json:"ts"`       // Unix timestamp detik
	CPU       float64 `json:"cpu"`      // Persentase penggunaan CPU (0-100%)
	RAMUsed   uint64  `json:"ram_used"` // Penggunaan RAM dalam byte
	RAMTotal  uint64  `json:"ram_total"`// Total RAM fisik dalam byte
	DiskUsed  uint64  `json:"disk_used"`// Penggunaan disk partisi root dalam byte
	DiskTotal uint64  `json:"disk_total"`// Total kapasitas disk dalam byte
	NetIn     uint64  `json:"net_in"`   // Kecepatan download (byte/detik)
	NetOut    uint64  `json:"net_out"`  // Kecepatan upload (byte/detik)
	LoadAvg   float64 `json:"load_avg"` // Load average 1 menit
	Load5m    float64 `json:"load_5m"`  // Load average 5 menit
	Load15m   float64 `json:"load_15m"` // Load average 15 menit
}
