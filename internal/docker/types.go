package docker

// DockerDiskUsageCategory merepresentasikan satu baris ringkasan dari `docker system df`
type DockerDiskUsageCategory struct {
	Type             string  `json:"type"`
	TotalCount       int     `json:"total_count"`
	ActiveCount      int     `json:"active_count"`
	SizeBytes        int64   `json:"size_bytes"`
	SizeHuman        string  `json:"size_human"`
	ReclaimableBytes int64   `json:"reclaimable_bytes"`
	ReclaimableHuman string  `json:"reclaimable_human"`
	ReclaimablePct   float64 `json:"reclaimable_pct"`
}

// DockerDiskUsage merangkum pemakaian disk Docker berdasarkan kategori (Images, Containers, Volumes, Build Cache)
type DockerDiskUsage struct {
	Images                DockerDiskUsageCategory `json:"images"`
	Containers            DockerDiskUsageCategory `json:"containers"`
	Volumes               DockerDiskUsageCategory `json:"volumes"`
	BuildCache            DockerDiskUsageCategory `json:"build_cache"`
	TotalSizeBytes        int64                   `json:"total_size_bytes"`
	TotalSizeHuman        string                  `json:"total_size_human"`
	TotalReclaimableBytes int64                   `json:"total_reclaimable_bytes"`
	TotalReclaimableHuman string                  `json:"total_reclaimable_human"`
}

// DockerStatus menyimpan ringkasan status runtime Docker Engine dan Docker Compose
type DockerStatus struct {
	Installed           bool             `json:"installed"`
	ComposeInstalled    bool             `json:"compose_installed"`
	Running             bool             `json:"running"`
	Version             string           `json:"version"`
	DockerVersion       string           `json:"docker_version"`
	ServerVersion       string           `json:"server_version"`
	ComposeVersion      string           `json:"compose_version"`
	ContainersTotal     int              `json:"containers_total"`
	ContainersCount     int              `json:"containers_count"`
	ContainersRunning   int              `json:"containers_running"`
	ContainersPaused    int              `json:"containers_paused"`
	ContainersStopped   int              `json:"containers_stopped"`
	ImagesTotal         int              `json:"images_total"`
	ImagesCount         int              `json:"images_count"`
	VolumesTotal        int              `json:"volumes_total"`
	VolumesCount        int              `json:"volumes_count"`
	NetworksTotal       int              `json:"networks_total"`
	NetworksCount       int              `json:"networks_count"`
	ComposeStacksTotal  int              `json:"compose_stacks_total"`
	StorageDriver       string           `json:"storage_driver"`
	RootDir             string           `json:"root_dir"`
	DataDir             string           `json:"data_dir"`
	UnixSocket          string           `json:"unix_socket"`
	OneClickInstallPath string           `json:"one_click_install_path"`
	MemoryLimit         bool             `json:"memory_limit"`
	SwapLimit           bool             `json:"swap_limit"`
	DiskUsage           *DockerDiskUsage `json:"disk_usage,omitempty"`
}

// DockerSystemInfo menyimpan detail perangkat keras dan lingkungan host untuk panel informasi Docker
type DockerSystemInfo struct {
	Hostname         string `json:"hostname"`
	OSVersion        string `json:"os_version"`
	SystemVersion    string `json:"system_version"`
	Architecture     string `json:"architecture"`
	KernelVersion    string `json:"kernel_version"`
	CPUCores         int    `json:"cpu_cores"`
	MemoryTotalBytes int64  `json:"memory_total_bytes"`
	MemorySize       string `json:"memory_size"`
	DockerVersion    string `json:"docker_version"`
	ComposeVersion   string `json:"compose_version"`
	StorageDriver    string `json:"storage_driver"`
	UnixAddress      string `json:"unix_address"`
	DataDirectory    string `json:"data_directory"`
}

// DockerContainerPort menyimpan pemetaan port host ke port internal kontainer
type DockerContainerPort struct {
	IP          string `json:"ip"`
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port"`
	Type        string `json:"type"`
}

// DockerContainer menyimpan detail kontainer Docker lengkap dengan metrik sumber daya real-time
type DockerContainer struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Names          []string              `json:"names"`
	Image          string                `json:"image"`
	ImageID        string                `json:"image_id"`
	Command        string                `json:"command"`
	State          string                `json:"state"`
	Status         string                `json:"status"`
	Created        int64                 `json:"created"`
	CreatedSince   string                `json:"created_since"`
	IPAddress      string                `json:"ip_address"`
	Ports          []DockerContainerPort `json:"ports"`
	Networks       []string              `json:"networks"`
	MountsCount    int                   `json:"mounts_count"`
	ComposeProject string                `json:"compose_project"`
	RestartPolicy  string                `json:"restart_policy"`
	CPUPercent     float64               `json:"cpu_percent"`
	MemUsageBytes  int64                 `json:"mem_usage_bytes"`
	MemLimitBytes  int64                 `json:"mem_limit_bytes"`
	MemUsage       string                `json:"mem_usage"`
	NetIO          string                `json:"net_io"`
	BlockIO        string                `json:"block_io"`
	SizeRW         string                `json:"size_rw"`
}

// DockerEnvItem merepresentasikan satu variabel lingkungan kontainer dengan penanda sensitif
type DockerEnvItem struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// DockerMountItem merepresentasikan volume atau bind mount pada kontainer
type DockerMountItem struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

// DockerNetworkAttachment merepresentasikan keterhubungan kontainer pada satu jaringan Docker
type DockerNetworkAttachment struct {
	Name       string `json:"name"`
	NetworkID  string `json:"network_id"`
	IPAddress  string `json:"ip_address"`
	Gateway    string `json:"gateway"`
	MacAddress string `json:"mac_address"`
}

// DockerContainerInspect menyimpan hasil inspeksi mendalam `docker inspect` terstruktur
type DockerContainerInspect struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Image          string                    `json:"image"`
	ImageID        string                    `json:"image_id"`
	State          string                    `json:"state"`
	Status         string                    `json:"status"`
	Running        bool                      `json:"running"`
	Paused         bool                      `json:"paused"`
	Restarting     bool                      `json:"restarting"`
	PID            int                       `json:"pid"`
	StartedAt      string                    `json:"started_at"`
	CreatedAt      string                    `json:"created_at"`
	RestartPolicy  string                    `json:"restart_policy"`
	RestartCount   int                       `json:"restart_count"`
	Platform       string                    `json:"platform"`
	Driver         string                    `json:"driver"`
	Hostname       string                    `json:"hostname"`
	WorkingDir     string                    `json:"working_dir"`
	Cmd            []string                  `json:"cmd"`
	Entrypoint     []string                  `json:"entrypoint"`
	Env            []DockerEnvItem           `json:"env"`
	Ports          []DockerContainerPort     `json:"ports"`
	Mounts         []DockerMountItem         `json:"mounts"`
	Networks       []DockerNetworkAttachment `json:"networks"`
	Labels         map[string]string         `json:"labels"`
	ComposeProject string                    `json:"compose_project"`
	HealthStatus   string                    `json:"health_status"`
}

// DockerImage menyimpan detail citra Docker yang tersimpan di host lokal
type DockerImage struct {
	ID           string `json:"id"`
	Repository   string `json:"repository"`
	Tag          string `json:"tag"`
	Size         int64  `json:"size"`
	SizeHuman    string `json:"size_human"`
	Created      int64  `json:"created"`
	CreatedSince string `json:"created_since"`
	Digest       string `json:"digest"`
	Containers   int    `json:"containers"`
	Dangling     bool   `json:"dangling"`
}

// DockerImageHistoryItem menyimpan satu layer dari hasil `docker image history`
type DockerImageHistoryItem struct {
	ID           string `json:"id"`
	CreatedSince string `json:"created_since"`
	CreatedBy    string `json:"created_by"`
	SizeBytes    int64  `json:"size_bytes"`
	SizeHuman    string `json:"size_human"`
	Comment      string `json:"comment"`
}

// DockerHubSearchItem menyimpan hasil pencarian repositori dari Docker Hub API v2
type DockerHubSearchItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	StarCount   int    `json:"star_count"`
	PullCount   int64  `json:"pull_count"`
	IsOfficial  bool   `json:"is_official"`
	IsAutomated bool   `json:"is_automated"`
}

// DockerHubTagItem menyimpan daftar tag versi beserta arsitektur dan ukuran dari Docker Hub API v2
type DockerHubTagItem struct {
	Name          string   `json:"name"`
	FullSize      int64    `json:"full_size"`
	SizeHuman     string   `json:"size_human"`
	LastUpdated   string   `json:"last_updated"`
	Digest        string   `json:"digest"`
	Architectures []string `json:"architectures"`
}

// DockerVolume menyimpan detail penyimpanan persisten volume Docker
type DockerVolume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at"`
	SizeBytes  int64             `json:"size_bytes"`
	SizeHuman  string            `json:"size_human"`
	RefCount   int               `json:"ref_count"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// DockerNetwork menyimpan detail jaringan virtual Docker beserta subnet dan kontainer terhubung
type DockerNetwork struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Driver          string   `json:"driver"`
	Scope           string   `json:"scope"`
	Subnet          string   `json:"subnet"`
	Gateway         string   `json:"gateway"`
	Internal        bool     `json:"internal"`
	Attachable      bool     `json:"attachable"`
	ContainersCount int      `json:"containers_count"`
	Containers      []string `json:"containers"`
}

// DockerComposeProject menyimpan struktur proyek stack Docker Compose (baik dikelola FikPanel maupun terdeteksi di host)
type DockerComposeProject struct {
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	RawStatus       string   `json:"raw_status"`
	ConfigPath      string   `json:"config_path"`
	WorkingDir      string   `json:"working_dir"`
	ContainersCount int      `json:"containers_count"`
	RunningCount    int      `json:"running_count"`
	YAMLContent     string   `json:"yaml_content"`
	Managed         bool     `json:"managed"`
	Services        []string `json:"services"`
}

// OneClickEnvParam mendeskripsikan satu input variabel lingkungan pada formulir pemasangan One-Click App
type OneClickEnvParam struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Default string `json:"default"`
	Type    string `json:"type"`
}

// OneClickTemplate menyimpan spesifikasi katalog One-Click Docker App yang selaras penuh antara backend dan frontend
type OneClickTemplate struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Category          string             `json:"category"`
	Description       string             `json:"description"`
	Icon              string             `json:"icon"`
	Image             string             `json:"image"`
	DefaultPort       int                `json:"default_port"`
	DefaultPorts      []string           `json:"default_ports"`
	DefaultVolumes    []string           `json:"default_volumes"`
	VolumeMount       string             `json:"volume_mount"`
	DefaultEnv        map[string]string  `json:"default_env"`
	EnvParams         []OneClickEnvParam `json:"env_params"`
	DockerComposeYAML string             `json:"docker_compose_yaml"`
	Readme            string             `json:"readme"`
}

// DockerRegistry menyimpan daftar kredensial registri privat atau publik yang terhubung ke Docker daemon
type DockerRegistry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Username  string `json:"username"`
	IsDefault bool   `json:"is_default"`
	LoggedIn  bool   `json:"logged_in"`
}

// DockerDaemonConfig menyimpan parameter konfigurasi /etc/docker/daemon.json
type DockerDaemonConfig struct {
	RegistryMirrors []string `json:"registry_mirrors"`
	ComposePath     string   `json:"compose_path"`
	PrivateRegistry string   `json:"private_registry"`
	LogRotation     bool     `json:"log_rotation"`
	LogMaxSize      string   `json:"log_max_size"`
	LogMaxFile      string   `json:"log_max_file"`
	IPv6            bool     `json:"ipv6"`
	IPv6Subnet      string   `json:"ipv6_subnet"`
	IPTables        bool     `json:"iptables"`
	LiveRestore     bool     `json:"live_restore"`
	OneClickDir     string   `json:"one_click_dir"`
	RawJSON         string   `json:"raw_json"`
}

// CreateContainerRequest adalah payload pembuatan kontainer Docker baru dari antarmuka Docker Studio
type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Ports         []string          `json:"ports"`
	Volumes       []string          `json:"volumes"`
	Env           map[string]string `json:"env"`
	Environment   []string          `json:"environment"`
	Network       string            `json:"network"`
	RestartPolicy string            `json:"restart_policy"`
	AutoRemove    bool              `json:"auto_remove"`
	Command       string            `json:"command"`
	MemoryLimitMB int               `json:"memory_limit_mb"`
	CPULimit      float64           `json:"cpu_limit"`
	PullFirst     bool              `json:"pull_first"`
}

// DeployTemplateRequest adalah payload pemasangan One-Click App Template
type DeployTemplateRequest struct {
	TemplateID  string            `json:"template_id"`
	ProjectName string            `json:"project_name"`
	Name        string            `json:"name"`
	Port        int               `json:"port"`
	Ports       map[string]string `json:"ports"`
	VolumePath  string            `json:"volume_path"`
	Env         map[string]string `json:"env"`
	EnvValues   map[string]string `json:"env_values"`
}

// ComposeActionRequest adalah payload kontrol siklus hidup stack Docker Compose
type ComposeActionRequest struct {
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
	Action     string `json:"action"` // up, down, restart, stop, start, pull
}

// SystemPruneRequest adalah payload pembersihan selektif sumber daya Docker usang
type SystemPruneRequest struct {
	Containers bool `json:"containers"`
	Images     bool `json:"images"`
	AllImages  bool `json:"all_images"`
	Volumes    bool `json:"volumes"`
	Networks   bool `json:"networks"`
	BuildCache bool `json:"build_cache"`
}

// SystemPruneResult merangkum hasil pembersihan ruang disk Docker
type SystemPruneResult struct {
	ReclaimedBytes int64    `json:"reclaimed_bytes"`
	ReclaimedHuman string   `json:"reclaimed_human"`
	Details        []string `json:"details"`
}

// BuildImageRequest adalah payload Dockerfile Quick Builder
type BuildImageRequest struct {
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
	NoCache    bool   `json:"no_cache"`
	Pull       bool   `json:"pull"`
}

// TagImageRequest adalah payload penandaan ulang (retag) citra lokal
type TagImageRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BulkContainerRequest adalah payload operasi massal terhadap banyak kontainer sekaligus
type BulkContainerRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"` // start, stop, restart, pause, unpause, remove
}

// BulkContainerResult melaporkan jumlah keberhasilan dan kegagalan operasi massal kontainer
type BulkContainerResult struct {
	Succeeded []string          `json:"succeeded"`
	Failed    map[string]string `json:"failed"`
}

// DockerStreamTaskSnapshot menyimpan ringkasan status tugas streaming Docker (pull, build, install)
type DockerStreamTaskSnapshot struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`   // pull, build, install, compose
	Target    string   `json:"target"` // misal: nginx:alpine
	Status    string   `json:"status"` // running, success, failed
	Error     string   `json:"error,omitempty"`
	StartedAt int64    `json:"started_at"`
	EndedAt   int64    `json:"ended_at,omitempty"`
	Logs      []string `json:"logs"`
}
