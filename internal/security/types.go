package security

// PortRule merepresentasikan satu aturan port firewall dengan status port aktif
type PortRule struct {
	Number    int    `json:"number"`
	Protocol  string `json:"protocol"` // tcp, udp, tcp/udp
	Port      string `json:"port"`     // misal: "22", "80", "39000-40000"
	Status    string `json:"status"`   // "Listening" atau "Not Listening"
	Strategy  string `json:"strategy"` // "Allow" atau "Deny"
	Direction string `json:"direction"`// "Inbound" atau "Outbound"
	SourceIP  string `json:"source_ip"`// "All" atau alamat IP/CIDR tertentu
	Remarks   string `json:"remarks"`  // Catatan keterangan aturan
	AddTime   string `json:"add_time"` // Waktu pembuatan aturan
}

// IPRule merepresentasikan aturan pemblokiran atau perizinan IP firewall
type IPRule struct {
	Number    int    `json:"number"`
	IP        string `json:"ip"`
	Strategy  string `json:"strategy"` // "Allow" atau "Deny"
	Direction string `json:"direction"`// "Inbound" atau "Outbound"
	Remarks   string `json:"remarks"`
	AddTime   string `json:"add_time"`
}

// FirewallExtendedStatus menyimpan informasi lengkap status firewall dan metrik aturan
type FirewallExtendedStatus struct {
	Enabled          bool       `json:"enabled"`
	BlockICMP        bool       `json:"block_icmp"`
	PortRulesCount   int        `json:"port_rules_count"`
	IPRulesCount     int        `json:"ip_rules_count"`
	PortForwardCount int        `json:"port_forward_count"`
	AreaRulesCount   int        `json:"area_rules_count"`
	Rules            []PortRule `json:"rules"`
	IPRules          []IPRule   `json:"ip_rules"`
	SiteLogsPath     string     `json:"site_logs_path"`
	SiteLogsSize     string     `json:"site_logs_size"`
	Raw              string     `json:"raw"`
}

// BruteForceConfig menyimpan konfigurasi proteksi serangan brute-force
type BruteForceConfig struct {
	Enabled               bool `json:"enabled"`
	UsernameProtection    bool `json:"username_protection"`
	UsernameMaxRetries    int  `json:"username_max_retries"`
	IPProtection          bool `json:"ip_protection"`
	IPMaxRetries          int  `json:"ip_max_retries"`
	IPBanDurationMinutes  int  `json:"ip_ban_duration_minutes"`
	RetentionMinutes      int  `json:"retention_minutes"`
}

// WhitelistIP merepresentasikan alamat IP yang masuk daftar putih
type WhitelistIP struct {
	IP      string `json:"ip"`
	Remarks string `json:"remarks"`
	AddedAt int64  `json:"added_at"`
}

// BlacklistIP merepresentasikan alamat IP yang diblokir oleh sistem
type BlacklistIP struct {
	IP        string `json:"ip"`
	Reason    string `json:"reason"`
	AddedAt   int64  `json:"added_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// FailedLoginEvent merepresentasikan catatan percobaan login yang gagal
type FailedLoginEvent struct {
	Time     int64  `json:"time"`
	IP       string `json:"ip"`
	Username string `json:"username"`
	Reason   string `json:"reason"`
	Source   string `json:"source"` // "panel" atau "ssh"
}

// SystemAccount merepresentasikan akun pengguna sistem operasi linux
type SystemAccount struct {
	Username string `json:"username"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Shell    string `json:"shell"`
	HomeDir  string `json:"home_dir"`
	Locked   bool   `json:"locked"`
}

// SSHStats merangkum metrik login SSH dan riwayat statistik
type SSHStats struct {
	ServiceActive     bool     `json:"service_active"`
	Port              int      `json:"port"`
	PasswordAuth      bool     `json:"password_auth"`
	KeyAuth           bool     `json:"key_auth"`
	PermitRootLogin   bool     `json:"permit_root_login"`
	SuccessfulLogins  int      `json:"successful_logins"`
	FailedLogins      int      `json:"failed_logins"`
	TodaySuccess      int      `json:"today_success"`
	TodayFail         int      `json:"today_fail"`
	YesterdaySuccess  int      `json:"yesterday_success"`
	YesterdayFail     int      `json:"yesterday_fail"`
	SevenDaysSuccess  []int    `json:"seven_days_success"`
	SevenDaysFail     []int    `json:"seven_days_fail"`
	RecentLogs        []SSHLog `json:"recent_logs"`
}

// SSHLog merepresentasikan riwayat log sesi autentikasi SSH
type SSHLog struct {
	Timestamp int64  `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Status    string `json:"status"` // "success" atau "failed"
	Message   string `json:"message"`
}

// SecurityChecklistItem merepresentasikan 10 poin evaluasi audit keamanan server
type SecurityChecklistItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // "safe", "warning", "danger"
	Fixable     bool   `json:"fixable"`
	ActionKey   string `json:"action_key"`
}

// LastLoginInfo menyimpan informasi sesi login terakhir
type LastLoginInfo struct {
	IP      string `json:"ip"`
	Time    string `json:"time"`
	Port    int    `json:"port"`
	Success bool   `json:"success"`
}

// ServerSecurityOverview menyimpan rangkuman skor keamanan dan 10 poin checklist
type ServerSecurityOverview struct {
	Score          int                     `json:"score"`
	Rating         string                  `json:"rating"` // "Secure", "Insecure", "Warning"
	LastSSHLogin   LastLoginInfo           `json:"last_ssh_login"`
	LastPanelLogin LastLoginInfo           `json:"last_panel_login"`
	Checklist      []SecurityChecklistItem `json:"checklist"`
}

// ProtectionOverview merangkum status aktif 6 pilar perlindungan utama sistem
type ProtectionOverview struct {
	Firewall           ProtectionItem `json:"firewall"`
	SSHProtection      ProtectionItem `json:"ssh_protection"`
	NginxWAF           ProtectionItem `json:"nginx_waf"`
	TamperProof        ProtectionItem `json:"tamper_proof"`
	IntrusionDetection ProtectionItem `json:"intrusion_detection"`
	Fail2Ban           ProtectionItem `json:"fail2ban"`
	EnabledCount       int            `json:"enabled_count"`
	TotalCount         int            `json:"total_count"`
	ClientIP           string         `json:"client_ip"`
	IsClientWhitelisted bool          `json:"is_client_whitelisted"`
}

// ProtectionItem merepresentasikan status detail satu modul keamanan
type ProtectionItem struct {
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Summary   string `json:"summary"`
}

// HardeningStatus menyimpan status pengerasan sistem operasi dan compiler
type HardeningStatus struct {
	CompilerBlocked      bool `json:"compiler_blocked"`
	ICMPBlocked          bool `json:"icmp_blocked"`
	SYNFloodProtected    bool `json:"syn_flood_protected"`
	IPSpoofingProtected  bool `json:"ip_spoofing_protected"`
	CoreDumpDisabled     bool `json:"core_dump_disabled"`
	TamperProofEnabled   bool `json:"tamper_proof_enabled"`
}
