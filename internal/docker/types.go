package docker

// ContainerPort merepresentasikan pemetaan port host ke container
type ContainerPort struct {
	IP          string `json:"ip"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
	Type        string `json:"type"` // tcp / udp
}

// DockerContainer merepresentasikan data kontainer Docker
type DockerContainer struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Image         string          `json:"image"`
	ImageID       string          `json:"image_id"`
	State         string          `json:"state"`  // running, exited, paused, restarting
	Status        string          `json:"status"` // misal: Up 2 hours
	Created       int64           `json:"created"`
	IPAddress     string          `json:"ip_address"`
	Ports         []ContainerPort `json:"ports"`
	CpuPercent    float64         `json:"cpu_percent"`
	MemUsageBytes int64           `json:"mem_usage_bytes"`
	MemLimitBytes int64           `json:"mem_limit_bytes"`
	Command       string          `json:"command"`
}

// DockerImage merepresentasikan citra kontainer lokal
type DockerImage struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       int64  `json:"size"`
	Created    int64  `json:"created"`
	Digest     string `json:"digest"`
}

// DockerVolume merepresentasikan penyimpanan persisten Docker
type DockerVolume struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	Scope      string `json:"scope"`
	SizeBytes  int64  `json:"size_bytes"`
	InUseCount int    `json:"in_use_count"`
	CreatedAt  string `json:"created_at"`
}

// DockerNetwork merepresentasikan jaringan virtual Docker
type DockerNetwork struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"` // bridge, host, overlay, macvlan
	Scope    string `json:"scope"`
	Subnet   string `json:"subnet"`
	Gateway  string `json:"gateway"`
	Internal bool   `json:"internal"`
}

// DockerComposeProject merepresentasikan proyek Docker Compose
type DockerComposeProject struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"` // running, partial, stopped
	ConfigFile    string   `json:"config_file"`
	WorkingDir    string   `json:"working_dir"`
	ServicesCount int      `json:"services_count"`
	Services      []string `json:"services"`
	RawYaml       string   `json:"raw_yaml"`
}

// OneClickTemplate merepresentasikan katalog template aplikasi instan (App Store)
type OneClickTemplate struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Category          string            `json:"category"` // database, web, tool, monitoring, dev
	Description       string            `json:"description"`
	Icon              string            `json:"icon"`
	Image             string            `json:"image"`
	DefaultPorts      []string          `json:"default_ports"`
	DefaultVolumes    []string          `json:"default_volumes"`
	DefaultEnv        map[string]string `json:"default_env"`
	DockerComposeYaml string            `json:"docker_compose_yaml"`
}

// DockerRegistry merepresentasikan kredensial registri privat yang tersimpan
type DockerRegistry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Username  string `json:"username"`
	IsDefault bool   `json:"is_default"`
}

// DockerStatus merepresentasikan status menyeluruh daemon Docker
type DockerStatus struct {
	Installed           bool    `json:"installed"`
	Running             bool    `json:"running"`
	DockerVersion       string  `json:"docker_version"`
	ComposeVersion      string  `json:"compose_version"`
	UnixSocket          string  `json:"unix_socket"`
	DataDir             string  `json:"data_dir"`
	StorageDriver       string  `json:"storage_driver"`
	ContainersCount     int     `json:"containers_count"`
	ContainersRunning   int     `json:"containers_running"`
	ContainersPaused    int     `json:"containers_paused"`
	ContainersStopped   int     `json:"containers_stopped"`
	ImagesCount         int     `json:"images_count"`
	VolumesCount        int     `json:"volumes_count"`
	NetworksCount       int     `json:"networks_count"`
	TotalCpuPercent     float64 `json:"total_cpu_percent"`
	TotalMemUsedBytes   int64   `json:"total_mem_used_bytes"`
	TotalMemLimitBytes  int64   `json:"total_mem_limit_bytes"`
	OneClickInstallPath string  `json:"one_click_install_path"`
}

// DockerSystemInfo merepresentasikan informasi rinci lingkungan host (Settings bagian bawah)
type DockerSystemInfo struct {
	Hostname             string `json:"hostname"`
	SystemVersion        string `json:"system_version"`
	Architecture         string `json:"architecture"`
	KernelVersion        string `json:"kernel_version"`
	CpuCore              int    `json:"cpu_core"`
	MemorySize           string `json:"memory_size"`
	DockerVersion        string `json:"docker_version"`
	DockerComposeVersion string `json:"docker_compose_version"`
	UnixAddress          string `json:"unix_address"`
	DataDirectory        string `json:"data_directory"`
}

// DockerDaemonConfig merepresentasikan konfigurasi daemon.json (Settings bagian atas)
type DockerDaemonConfig struct {
	AccelerateURL       string `json:"accelerate_url"`
	DockerComposePath   string `json:"docker_compose_path"`
	PrivateRegistry     string `json:"private_registry"`
	LogRotation         bool   `json:"log_rotation"`
	LogMaxSize          string `json:"log_max_size"`
	LogMaxFile          string `json:"log_max_file"`
	Ipv6                bool   `json:"ipv6"`
	Iptables            bool   `json:"iptables"`
	LiveRestore         bool   `json:"live_restore"`
	RawConfigJSON       string `json:"raw_config_json"`
	OneClickInstallDir  string `json:"one_click_install_dir"`
}

// CreateContainerRequest mendefinisikan muatan pembuatan kontainer baru
type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Ports         []string          `json:"ports"`   // misal "8080:80", "3000:3000"
	Volumes       []string          `json:"volumes"` // misal "/var/www:/app", "data:/data"
	Env           map[string]string `json:"env"`
	Network       string            `json:"network"`
	RestartPolicy string            `json:"restart_policy"` // no, always, on-failure, unless-stopped
	MemoryLimitMB int64             `json:"memory_limit_mb"`
	CpuLimit      float64           `json:"cpu_limit"`
	AutoStart     bool              `json:"auto_start"`
}

// DeployTemplateRequest mendefinisikan permintaan deploy aplikasi instan
type DeployTemplateRequest struct {
	TemplateID string            `json:"template_id"`
	ProjectName string           `json:"project_name"`
	Ports       map[string]string `json:"ports"`
	Volumes     map[string]string `json:"volumes"`
	Env         map[string]string `json:"env"`
}

// ImageSearchItem merepresentasikan hasil pencarian Docker Hub
type ImageSearchItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	IsOfficial  bool   `json:"is_official"`
	IsAutomated bool   `json:"is_automated"`
}
