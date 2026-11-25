package security

// FirewallRule merepresentasikan satu aturan firewall UFW aktual (inbound atau outbound)
type FirewallRule struct {
	Number    int    `json:"number"`
	To        string `json:"to"`
	Port      string `json:"port"`
	Protocol  string `json:"protocol"`   // tcp, udp, any
	Action    string `json:"action"`     // ALLOW, DENY, REJECT, LIMIT
	Direction string `json:"direction"`  // Inbound, Outbound
	From      string `json:"from"`       // Alamat sumber / CIDR / Anywhere
	IPVersion string `json:"ip_version"` // IPv4 atau IPv6
	Service   string `json:"service"`    // Label layanan terdeteksi (SSH, HTTP, MySQL, dll.)
	Remarks   string `json:"remarks"`    // Komentar UFW asli (# ...)
	Listening bool   `json:"listening"`  // Apakah ada proses aktif mendengar di port ini
	Process   string `json:"process"`    // Nama proses yang mendengar di port ini (bila ada)
	Protected bool   `json:"protected"`  // True bila port kritikal (SSH aktif / port panel)
}

// ListeningPort merepresentasikan soket jaringan aktif dari ss -tulnp
type ListeningPort struct {
	Protocol      string `json:"protocol"`       // tcp, tcp6, udp, udp6
	Address       string `json:"address"`        // Alamat bind (0.0.0.0, ::, 127.0.0.1, ::1)
	Port          string `json:"port"`           // Nomor port
	Process       string `json:"process"`        // Nama biner proses
	PID           int    `json:"pid"`            // ID proses
	ExposedPublic bool   `json:"exposed_public"` // True bila bind ke semua interface publik
	Service       string `json:"service"`        // Label layanan terdeteksi
}

// FirewallStatus menyimpan status ringkas UFW (kompatibilitas endpoint dasar)
type FirewallStatus struct {
	Enabled         bool           `json:"enabled"`
	Installed       bool           `json:"installed"`
	DefaultIncoming string         `json:"default_incoming"`
	DefaultOutgoing string         `json:"default_outgoing"`
	Rules           []FirewallRule `json:"rules"`
	Raw             string         `json:"raw"`
}

// FirewallExtendedStatus menyimpan status menyeluruh UFW, kebijakan default, soket aktif, dan ukuran log web
type FirewallExtendedStatus struct {
	IsLinux          bool            `json:"is_linux"`
	Installed        bool            `json:"installed"`
	Enabled          bool            `json:"enabled"`
	Hostname         string          `json:"hostname"`
	DefaultIncoming  string          `json:"default_incoming"` // deny, allow, reject
	DefaultOutgoing  string          `json:"default_outgoing"` // allow, deny, reject
	BlockICMP        bool            `json:"block_icmp"`
	SSHPort          int             `json:"ssh_port"`
	PanelPort        int             `json:"panel_port"`
	InboundCount     int             `json:"inbound_count"`
	OutboundCount    int             `json:"outbound_count"`
	ListeningCount   int             `json:"listening_count"`
	Rules            []FirewallRule  `json:"rules"`
	ListeningPorts   []ListeningPort `json:"listening_ports"`
	SiteLogsPath     string          `json:"site_logs_path"`
	SiteLogsSize     string          `json:"site_logs_size"`
	SiteLogsBytes    int64           `json:"site_logs_bytes"`
	Raw              string          `json:"raw"`
}

// AddFirewallRuleInput menampung payload pembuatan aturan firewall baru
type AddFirewallRuleInput struct {
	Port      string `json:"port"`
	Protocol  string `json:"protocol"`  // tcp, udp, any
	Action    string `json:"action"`    // allow, deny, reject, limit
	Direction string `json:"direction"` // in, out
	SourceIP  string `json:"source_ip"` // 0.0.0.0/0, Anywhere, atau IP/CIDR spesifik
	Remarks   string `json:"remarks"`
}

// SSHConfig merepresentasikan konfigurasi daemon OpenSSH aktual pada server
type SSHConfig struct {
	IsLinux            bool   `json:"is_linux"`
	Installed          bool   `json:"installed"`
	ActiveService      bool   `json:"active_service"`
	Port               int    `json:"port"`
	PasswordAuth       bool   `json:"password_auth"`
	PubkeyAuth         bool   `json:"pubkey_auth"`
	PermitRootLogin    string `json:"permit_root_login"` // yes, prohibit-password, no
	MaxAuthTries       int    `json:"max_auth_tries"`
	CloudInitOverride  bool   `json:"cloud_init_override"`
	CloudInitFile      string `json:"cloud_init_file,omitempty"`
	ConfigSource       string `json:"config_source"`
	AuthorizedKeyCount int    `json:"authorized_key_count"`
}

// SSHConfigUpdate menampung payload pembaruan konfigurasi OpenSSH
type SSHConfigUpdate struct {
	Port            int    `json:"port"`
	PasswordAuth    bool   `json:"password_auth"`
	PubkeyAuth      bool   `json:"pubkey_auth"`
	PermitRootLogin string `json:"permit_root_login"` // yes, prohibit-password, no
	MaxAuthTries    int    `json:"max_auth_tries"`
	Force           bool   `json:"force"` // Abaikan peringatan tanpa SSH key bila dikonfirmasi eksplisit
}

// SSHAuthorizedKey merepresentasikan satu kunci publik di berkas ~/.ssh/authorized_keys
type SSHAuthorizedKey struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	Preview     string `json:"preview"`
}

// SSHLog merepresentasikan satu baris kejadian autentikasi SSH aktual dari journald/auth.log
type SSHLog struct {
	Timestamp int64  `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Status    string `json:"status"` // success, failed
	Method    string `json:"method"` // publickey, password, invalid_user, dll.
	Message   string `json:"message"`
}

// SSHDayStat merangkum jumlah login SSH sukses dan gagal untuk satu hari kalender
type SSHDayStat struct {
	Date    string `json:"date"`  // YYYY-MM-DD
	Label   string `json:"label"` // Mis. "Sen", "24 Nov"
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// SSHStats merangkum konfigurasi, statistik 7 hari, dan riwayat log autentikasi SSH
type SSHStats struct {
	Config           SSHConfig    `json:"config"`
	LogSource        string       `json:"log_source"`
	SuccessfulLogins int          `json:"successful_logins"`
	FailedLogins     int          `json:"failed_logins"`
	TodaySuccess     int          `json:"today_success"`
	TodayFail        int          `json:"today_fail"`
	YesterdaySuccess int          `json:"yesterday_success"`
	YesterdayFail    int          `json:"yesterday_fail"`
	DailyStats       []SSHDayStat `json:"daily_stats"`
	RecentLogs       []SSHLog     `json:"recent_logs"`
}

// SystemAccount merepresentasikan akun pengguna sistem operasi Linux
type SystemAccount struct {
	Username    string `json:"username"`
	UID         int    `json:"uid"`
	GID         int    `json:"gid"`
	Shell       string `json:"shell"`
	HomeDir     string `json:"home_dir"`
	Locked      bool   `json:"locked"`
	LockReason  string `json:"lock_reason"` // "active", "passwd_locked", "nologin_shell"
	IsProtected bool   `json:"is_protected"`
}

// BruteForceConfig menyimpan konfigurasi proteksi brute-force panel persisten
type BruteForceConfig struct {
	Enabled              bool `json:"enabled"`
	UsernameProtection   bool `json:"username_protection"`
	UsernameMaxRetries   int  `json:"username_max_retries"`
	IPProtection         bool `json:"ip_protection"`
	IPMaxRetries         int  `json:"ip_max_retries"`
	IPBanDurationMinutes int  `json:"ip_ban_duration_minutes"`
	RetentionMinutes     int  `json:"retention_minutes"`
	NotifyOnLogin        bool `json:"notify_on_login"`
	NotifyOnBan          bool `json:"notify_on_ban"`
	AutoSyncUFW          bool `json:"auto_sync_ufw"`
}

// Fail2BanJail merepresentasikan metrik satu jail Fail2Ban aktual
type Fail2BanJail struct {
	Name          string   `json:"name"`
	CurrentlyFail int      `json:"currently_failed"`
	TotalFail     int      `json:"total_failed"`
	BannedCount   int      `json:"banned_count"`
	BannedIPs     []string `json:"banned_ips"`
}

// Fail2BanStatus merepresentasikan status instalasi dan layanan daemon Fail2Ban
type Fail2BanStatus struct {
	IsLinux   bool           `json:"is_linux"`
	Installed bool           `json:"installed"`
	Running   bool           `json:"running"`
	Jails     []Fail2BanJail `json:"jails"`
}

// SysctlParam merepresentasikan satu parameter pengerasan kernel Linux aktual dari /proc/sys
type SysctlParam struct {
	Key              string `json:"key"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Enabled          bool   `json:"enabled"`
	CurrentValue     string `json:"current_value"`
	RecommendedValue string `json:"recommended_value"`
	Persisted        bool   `json:"persisted"`
}

// HardeningStatus merangkum seluruh status pengerasan kernel, compiler, dan FIM
type HardeningStatus struct {
	IsLinux          bool          `json:"is_linux"`
	CompilerBlocked  bool          `json:"compiler_blocked"`
	CompilersFound   []string      `json:"compilers_found"`
	ICMPBlocked      bool          `json:"icmp_blocked"`
	SysctlParams     []SysctlParam `json:"sysctl_params"`
	FIMEnabled       bool          `json:"fim_enabled"`
	FIMBaselineCount int           `json:"fim_baseline_count"`
	FIMLastUpdated   int64         `json:"fim_last_updated"`
}

// FIMEntryStatus merepresentasikan hasil verifikasi integritas satu berkas kritis
type FIMEntryStatus struct {
	Path         string `json:"path"`
	Status       string `json:"status"` // "ok", "modified", "missing"
	ExpectedHash string `json:"expected_hash"`
	ActualHash   string `json:"actual_hash"`
	Size         int64  `json:"size"`
	Mode         string `json:"mode"`
	ModTime      int64  `json:"mod_time"`
}

// FIMCheckResult merangkum hasil audit integritas berkas kritis terhadap baseline SHA-256
type FIMCheckResult struct {
	Enabled        bool             `json:"enabled"`
	TotalFiles     int              `json:"total_files"`
	OKCount        int              `json:"ok_count"`
	ModifiedCount  int              `json:"modified_count"`
	MissingCount   int              `json:"missing_count"`
	LastBaselineAt int64            `json:"last_baseline_at"`
	CheckedAt      int64            `json:"checked_at"`
	Items          []FIMEntryStatus `json:"items"`
}

// ScanFinding merepresentasikan satu item hasil audit atau pemindaian keamanan server
type ScanFinding struct {
	ID        string `json:"id"`
	Category  string `json:"category"` // baseline, risk, malware, vuln
	Severity  string `json:"severity"` // critical, warning, safe
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	FixHint   string `json:"fix_hint"`
	TargetTab string `json:"target_tab"` // overview, firewall, ssh, bruteforce, hardening, settings
	ActionKey string `json:"action_key,omitempty"`
	Fixable   bool   `json:"fixable"`
}

// SecurityScanReport menyimpan laporan komprehensif hasil pemindaian keamanan server
type SecurityScanReport struct {
	ID            int64         `json:"id,omitempty"`
	Score         int           `json:"score"`
	Rating        string        `json:"rating"` // Aman, Perlu Perhatian, Berisiko
	ScannedAt     int64         `json:"scanned_at"`
	RiskCount     int           `json:"risk_count"`
	MalwareCount  int           `json:"malware_count"`
	VulnCount     int           `json:"vuln_count"`
	BaselinePass  int           `json:"baseline_pass"`
	BaselineTotal int           `json:"baseline_total"`
	Findings      []ScanFinding `json:"findings"`
}

// ScanOptions menyediakan konteks panel & layanan dari Server untuk evaluasi audit yang akurat
type ScanOptions struct {
	IsHTTPS           bool
	TOTPEnabled       bool
	PasskeyCount      int
	IPAllowlistCount  int
	WAFEnabled        bool
	BruteForceEnabled bool
	SiteRoots         []string
	ExpiringSSLCount  int
	ClientIP          string
}

// LastLoginInfo menyimpan catatan login terakhir SSH atau Panel secara aktual
type LastLoginInfo struct {
	Available bool   `json:"available"`
	IP        string `json:"ip"`
	User      string `json:"user"`
	Time      string `json:"time"`
	Timestamp int64  `json:"timestamp"`
	Port      int    `json:"port,omitempty"`
	Success   bool   `json:"success"`
}

// ProtectionItem merepresentasikan status aktual satu pilar perlindungan di halaman Overview
type ProtectionItem struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Enabled    bool   `json:"enabled"`
	StatusText string `json:"status_text"`
	Summary    string `json:"summary"`
	TargetTab  string `json:"target_tab"`
	Toggleable bool   `json:"toggleable"`
}

// ProtectionOverview merangkum postur keamanan lengkap untuk tab Overview
type ProtectionOverview struct {
	IsLinux             bool                `json:"is_linux"`
	Hostname            string              `json:"hostname"`
	EnabledCount        int                 `json:"enabled_count"`
	TotalCount          int                 `json:"total_count"`
	ClientIP            string              `json:"client_ip"`
	IsClientWhitelisted bool                `json:"is_client_whitelisted"`
	LastSSHLogin        LastLoginInfo       `json:"last_ssh_login"`
	LastPanelLogin      LastLoginInfo       `json:"last_panel_login"`
	LatestScan          *SecurityScanReport `json:"latest_scan"`
	Pillars             []ProtectionItem    `json:"pillars"`
}
