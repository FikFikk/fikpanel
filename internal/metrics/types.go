package metrics

// Point merepresentasikan satu titik telemetri sistem pada waktu tertentu
type Point struct {
	Timestamp int64     `json:"ts"`                   // Unix timestamp detik
	CPU       float64   `json:"cpu"`                  // Persentase total penggunaan CPU (0-100%)
	CPUUser   float64   `json:"cpu_user,omitempty"`   // Persentase CPU mode user + nice
	CPUSystem float64   `json:"cpu_system,omitempty"` // Persentase CPU mode kernel + irq
	CPUIOWait float64   `json:"cpu_iowait,omitempty"` // Persentase CPU menunggu I/O disk
	CPUSteal  float64   `json:"cpu_steal,omitempty"`  // Persentase CPU steal hypervisor VPS
	PerCore   []float64 `json:"per_core,omitempty"`   // Persentase penggunaan per core CPU
	RAMUsed   uint64    `json:"ram_used"`             // Penggunaan RAM dalam byte
	RAMTotal  uint64    `json:"ram_total"`            // Total RAM fisik dalam byte
	DiskUsed  uint64    `json:"disk_used"`            // Penggunaan disk partisi root dalam byte
	DiskTotal uint64    `json:"disk_total"`           // Total kapasitas disk dalam byte
	DiskRead  uint64    `json:"disk_read,omitempty"`  // Kecepatan baca disk (byte/detik)
	DiskWrite uint64    `json:"disk_write,omitempty"` // Kecepatan tulis disk (byte/detik)
	NetIn     uint64    `json:"net_in"`               // Kecepatan download (byte/detik)
	NetOut    uint64    `json:"net_out"`              // Kecepatan upload (byte/detik)
	LoadAvg   float64   `json:"load_avg"`             // Load average 1 menit
	Load5m    float64   `json:"load_5m"`              // Load average 5 menit
	Load15m   float64   `json:"load_15m"`             // Load average 15 menit
}
