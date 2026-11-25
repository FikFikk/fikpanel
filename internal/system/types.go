package system

// DiskPartition merepresentasikan satu partisi mountpoint fisik beserta penggunaan ruang dan inode
type DiskPartition struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	FSType      string  `json:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	AvailBytes  uint64  `json:"avail_bytes"`
	UsedPct     float64 `json:"used_pct"`
	InodesTotal uint64  `json:"inodes_total"`
	InodesUsed  uint64  `json:"inodes_used"`
	InodesPct   float64 `json:"inodes_pct"`
}

// HostOverview merangkum telemetri perangkat keras, beban CPU, pecahan memori, proses, dan partisi disk
type HostOverview struct {
	IsLinux        bool            `json:"is_linux"`
	Hostname       string          `json:"hostname"`
	OS             string          `json:"os"`
	Kernel         string          `json:"kernel"`
	Arch           string          `json:"arch"`
	CPUModel       string          `json:"cpu_model"`
	CPUCores       int             `json:"cpu_cores"`
	UptimeSec      int64           `json:"uptime_sec"`
	Load1          float64         `json:"load_1"`
	Load5          float64         `json:"load_5"`
	Load15         float64         `json:"load_15"`
	MemTotalBytes  uint64          `json:"mem_total_bytes"`
	MemUsedBytes   uint64          `json:"mem_used_bytes"`
	MemAvailBytes  uint64          `json:"mem_avail_bytes"`
	MemCacheBytes  uint64          `json:"mem_cache_bytes"`
	MemUsedPct     float64         `json:"mem_used_pct"`
	SwapTotalBytes uint64          `json:"swap_total_bytes"`
	SwapUsedBytes  uint64          `json:"swap_used_bytes"`
	SwapUsedPct    float64         `json:"swap_used_pct"`
	TotalProcs     int             `json:"total_procs"`
	RunningProcs   int             `json:"running_procs"`
	ZombieProcs    int             `json:"zombie_procs"`
	OOMKillsRecent int             `json:"oom_kills_recent"`
	NeedsReboot    bool            `json:"needs_reboot"`
	Disks          []DiskPartition `json:"disks"`
}

// ProcessInfo merepresentasikan satu baris proses OS dari `ps`
type ProcessInfo struct {
	PID       int     `json:"pid"`
	PPID      int     `json:"ppid"`
	User      string  `json:"user"`
	State     string  `json:"state"`   // R, S, D, Z, T, I
	CPU       float64 `json:"cpu"`     // Persentase CPU
	MemPct    float64 `json:"mem_pct"` // Persentase RAM
	Memory    float64 `json:"memory"`  // RSS dalam MB
	Threads   int     `json:"threads"`
	Elapsed   string  `json:"elapsed"`
	Command   string  `json:"command"`
	Protected bool    `json:"protected"` // True bila proses vital (PID 1, fikpanel, systemd, sshd)
}

// ProcessDetail menyajikan inspeksi mendalam satu proses dari /proc/<pid>
type ProcessDetail struct {
	PID       int    `json:"pid"`
	PPID      int    `json:"ppid"`
	User      string `json:"user"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Cmdline   string `json:"cmdline"`
	Exe       string `json:"exe"`
	Cwd       string `json:"cwd"`
	RSSBytes  uint64 `json:"rss_bytes"`
	SwapBytes uint64 `json:"swap_bytes"`
	Threads   int    `json:"threads"`
	FDCount   int    `json:"fd_count"`
	Nice      int    `json:"nice"`
	OOMScore  int    `json:"oom_score"`
	Protected bool   `json:"protected"`
}

// PortListening merepresentasikan soket TCP/UDP yang mendengarkan koneksi beserta proses pemiliknya
type PortListening struct {
	Proto         string `json:"proto"` // tcp, tcp6, udp, udp6
	Port          string `json:"port"`
	Address       string `json:"address"`
	Program       string `json:"program"`
	PID           int    `json:"pid"`
	ExposedPublic bool   `json:"exposed_public"`
	Service       string `json:"service"`
	CanKill       bool   `json:"can_kill"`
}

// NetworkOverview merangkum daftar port listening dan distribusi status koneksi TCP aktif
type NetworkOverview struct {
	IsLinux     bool            `json:"is_linux"`
	Ports       []PortListening `json:"ports"`
	Connections map[string]int  `json:"connections"`
}

// SwapDevice merepresentasikan satu perangkat atau berkas swap aktif di /proc/swaps
type SwapDevice struct {
	Filename  string `json:"filename"`
	Type      string `json:"type"`
	SizeBytes uint64 `json:"size_bytes"`
	UsedBytes uint64 `json:"used_bytes"`
	Priority  int    `json:"priority"`
}

// MemoryOptimizationStatus menyimpan informasi RAM, swap aktif, dan parameter kernel memori
type MemoryOptimizationStatus struct {
	IsLinux          bool         `json:"is_linux"`
	MemTotalBytes    uint64       `json:"mem_total_bytes"`
	MemUsedBytes     uint64       `json:"mem_used_bytes"`
	MemAvailBytes    uint64       `json:"mem_avail_bytes"`
	MemCacheBytes    uint64       `json:"mem_cache_bytes"`
	SwapTotalBytes   uint64       `json:"swap_total_bytes"`
	SwapUsedBytes    uint64       `json:"swap_used_bytes"`
	Swappiness       int          `json:"swappiness"`
	VFSCachePressure int          `json:"vfs_cache_pressure"`
	Swaps            []SwapDevice `json:"swaps"`
	FikPanelSwapPath string       `json:"fikpanel_swap_path"`
	HasFikPanelSwap  bool         `json:"has_fikpanel_swap"`
	RootFreeBytes    uint64       `json:"root_free_bytes"`
}

// CleanerCategory merepresentasikan satu kategori berkas sampah yang dipindai
type CleanerCategory struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
}

// CleanerScanResult merangkum hasil pemindaian seluruh kategori sampah & cache host
type CleanerScanResult struct {
	IsLinux        bool              `json:"is_linux"`
	AptCacheBytes  int64             `json:"apt_cache_bytes"`
	JournalBytes   int64             `json:"journal_bytes"`
	SystemLogBytes int64             `json:"system_log_bytes"`
	TmpBytes       int64             `json:"tmp_bytes"`
	DevCacheBytes  int64             `json:"dev_cache_bytes"`
	DockerReclaim  int64             `json:"docker_reclaim_bytes"`
	TotalBytes     int64             `json:"total_bytes"`
	ScannedAt      int64             `json:"scanned_at"`
	Categories     []CleanerCategory `json:"categories"`
}

// CleanRequest menentukan kategori pembersihan yang dipilih pengguna
type CleanRequest struct {
	CleanApt      bool `json:"clean_apt"`
	CleanJournal  bool `json:"clean_journal"`
	CleanLogs     bool `json:"clean_logs"`
	CleanTmp      bool `json:"clean_tmp"`
	CleanDevCache bool `json:"clean_dev_cache"`
	CleanDocker   bool `json:"clean_docker"`
	DropCaches    bool `json:"drop_caches"`
}

// CleanReport melaporkan hasil nyata ruang disk & RAM yang berhasil dibebaskan
type CleanReport struct {
	BeforeTotal    int64    `json:"before_total"`
	AfterTotal     int64    `json:"after_total"`
	ReclaimedBytes int64    `json:"reclaimed_bytes"`
	RAMFreedBytes  int64    `json:"ram_freed_bytes"`
	Actions        []string `json:"actions"`
	Errors         []string `json:"errors"`
}

// DiskHogEntry merepresentasikan satu direktori terbesar hasil analisis ruang disk
type DiskHogEntry struct {
	Path      string  `json:"path"`
	SizeBytes int64   `json:"size_bytes"`
	Percent   float64 `json:"percent"`
}

// DiskHogsResult menampung daftar direktori terbesar pada suatu target root
type DiskHogsResult struct {
	IsLinux    bool           `json:"is_linux"`
	TargetPath string         `json:"target_path"`
	TotalBytes int64          `json:"total_bytes"`
	Entries    []DiskHogEntry `json:"entries"`
}

// PackageUpdate merepresentasikan satu paket OS yang memiliki pembaruan tersedia
type PackageUpdate struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
	Source         string `json:"source"`
	IsSecurity     bool   `json:"is_security"`
}

// UpdatesOverview merangkum daftar paket OS tertunda beserta penanda kebutuhan reboot
type UpdatesOverview struct {
	IsLinux        bool            `json:"is_linux"`
	PackageManager string          `json:"package_manager"`
	TotalCount     int             `json:"total_count"`
	SecurityCount  int             `json:"security_count"`
	NeedsReboot    bool            `json:"needs_reboot"`
	CheckedAt      int64           `json:"checked_at"`
	Updates        []PackageUpdate `json:"updates"`
}

// DiagnosticLogEntry merepresentasikan satu baris log galat/peringatan kernel atau systemd
type DiagnosticLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // oom, error, warning
	Source    string `json:"source"`
	Message   string `json:"message"`
	IsOOM     bool   `json:"is_oom"`
}

// DiagnosticsReport merangkum log galat kernel, peringatan systemd, dan insiden OOM Killer
type DiagnosticsReport struct {
	IsLinux    bool                 `json:"is_linux"`
	OOMCount   int                  `json:"oom_count"`
	ErrorCount int                  `json:"error_count"`
	WarnCount  int                  `json:"warn_count"`
	Entries    []DiagnosticLogEntry `json:"entries"`
}
