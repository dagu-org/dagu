package config

// Definition holds the overall configuration for the application.
// Fields are organized into logical groups for clarity.
type Definition struct {
	// Server settings
	Host        string  `mapstructure:"host"`
	Port        int     `mapstructure:"port"`
	BasePath    string  `mapstructure:"base_path"`
	APIBasePath string  `mapstructure:"api_base_path"`
	APIBaseURL  string  `mapstructure:"api_base_url"` // Deprecated: use APIBasePath
	Headless    *bool   `mapstructure:"headless"`
	TLS         *TLSDef `mapstructure:"tls"`

	// Core settings
	Debug        bool   `mapstructure:"debug"`
	DefaultShell string `mapstructure:"default_shell"`
	LogFormat    string `mapstructure:"log_format"`  // "json" or "text"
	AccessLog    *string `mapstructure:"access_log"` // "all" (default), "non-public", or "none"
	TZ           string `mapstructure:"tz"`

	// Authentication
	Auth *AuthDef `mapstructure:"auth"`

	// Permissions
	PermissionWriteDAGs *bool          `mapstructure:"permission_write_dags"`
	PermissionRunDAGs   *bool          `mapstructure:"permission_run_dags"`
	Permissions         PermissionsDef `mapstructure:"permissions"`

	// Paths (legacy flat fields)
	DAGs            string `mapstructure:"dags"` // Deprecated
	DAGsDir         string `mapstructure:"dags_dir"`
	Executable      string `mapstructure:"executable"`
	LogDir          string `mapstructure:"log_dir"`
	DataDir         string `mapstructure:"data_dir"`
	SuspendFlagsDir string `mapstructure:"suspend_flags_dir"`
	AdminLogsDir    string `mapstructure:"admin_logs_dir"`
	BaseConfig      string `mapstructure:"base_config"`

	// Paths (structured)
	Paths *PathsDef `mapstructure:"paths"`

	// UI settings (legacy flat fields)
	LogEncodingCharset    string `mapstructure:"log_encoding_charset"`
	NavbarColor           string `mapstructure:"navbar_color"`
	NavbarTitle           string `mapstructure:"navbar_title"`
	MaxDashboardPageLimit int    `mapstructure:"max_dashboard_page_limit"`
	LatestStatusToday     *bool  `mapstructure:"latest_status_today"`

	// UI settings (structured)
	UI *UIDef `mapstructure:"ui"`

	// Peer connections
	Peer PeerDef `mapstructure:"peer"`

	// Remote nodes
	RemoteNodes []RemoteNodeDef `mapstructure:"remote_nodes"`

	// Services
	Coordinator *CoordinatorDef `mapstructure:"coordinator"`
	Worker      *WorkerDef      `mapstructure:"worker"`
	Scheduler   *SchedulerDef   `mapstructure:"scheduler"`
	Queues      *QueueConfigDef `mapstructure:"queues"`

	// Execution
	DefaultExecutionMode string `mapstructure:"default_execution_mode"`

	// Features
	Monitoring *MonitoringDef `mapstructure:"monitoring"`
	Metrics    *string        `mapstructure:"metrics"` // "public" or "private"
	Cache      *string        `mapstructure:"cache"`   // "low", "normal", or "high"
	Terminal   *TerminalDef   `mapstructure:"terminal"`
	Audit      *AuditDef      `mapstructure:"audit"`
	Session    *SessionDef    `mapstructure:"session"`
	GitSync    *GitSyncDef    `mapstructure:"git_sync"`
	Tunnel     *TunnelDef     `mapstructure:"tunnel"`
}

// -----------------------------------------------------------------------------
// Server Configuration
// -----------------------------------------------------------------------------

// TLSDef configures TLS/SSL encryption.
type TLSDef struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

// -----------------------------------------------------------------------------
// Authentication Configuration
// -----------------------------------------------------------------------------

// AuthDef configures authentication for the application.
type AuthDef struct {
	Mode    *string         `mapstructure:"mode"` // "none", "basic", or "builtin"
	Basic   *AuthBasicDef   `mapstructure:"basic"`
	OIDC    *AuthOIDCDef    `mapstructure:"oidc"`
	Builtin *AuthBuiltinDef `mapstructure:"builtin"`
}

// AuthBasicDef configures basic authentication credentials.
type AuthBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthBuiltinDef configures builtin authentication with RBAC.
type AuthBuiltinDef struct {
	Token *TokenConfigDef `mapstructure:"token"`
}

// TokenConfigDef configures JWT token settings.
type TokenConfigDef struct {
	Secret string `mapstructure:"secret"`
	TTL    string `mapstructure:"ttl"`
}

// AuthOIDCDef configures OIDC authentication.
// These fields are used when auth.mode=builtin with an OIDC provider configured.
type AuthOIDCDef struct {
	// ClientID is the OAuth client identifier (Go naming: ID not Id).
	// mapstructure tag uses lowercase "client_id" for YAML compatibility.
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	// ClientURL is the application callback URL (Go naming: URL not Url).
	// mapstructure tag uses lowercase "client_url" for YAML compatibility.
	ClientURL      string              `mapstructure:"client_url"`
	Issuer         string              `mapstructure:"issuer"`
	Scopes         []string            `mapstructure:"scopes"`
	Whitelist      []string            `mapstructure:"whitelist"`
	AutoSignup     *bool               `mapstructure:"auto_signup"` // Default: true (builtin mode only)
	AllowedDomains []string            `mapstructure:"allowed_domains"`
	ButtonLabel    string              `mapstructure:"button_label"`
	RoleMapping    *OIDCRoleMappingDef `mapstructure:"role_mapping"`
}

// OIDCRoleMappingDef maps OIDC claims to Dagu roles.
type OIDCRoleMappingDef struct {
	DefaultRole         string            `mapstructure:"default_role"`          // Default: "viewer"
	GroupsClaim         string            `mapstructure:"groups_claim"`          // Default: "groups"
	GroupMappings       map[string]string `mapstructure:"group_mappings"`        // IdP group -> Dagu role
	RoleAttributePath   string            `mapstructure:"role_attribute_path"`   // jq expression for role extraction
	RoleAttributeStrict *bool             `mapstructure:"role_attribute_strict"` // Deny login if no valid role found
	SkipOrgRoleSync     *bool             `mapstructure:"skip_org_role_sync"`    // Only assign roles on first login
}

// PermissionsDef configures UI and API permissions.
type PermissionsDef struct {
	WriteDAGs *bool `mapstructure:"write_dags"`
	RunDAGs   *bool `mapstructure:"run_dags"`
}

// -----------------------------------------------------------------------------
// Path Configuration
// -----------------------------------------------------------------------------

// PathsDef configures file system paths.
type PathsDef struct {
	DAGsDir            string `mapstructure:"dags_dir"`
	Executable         string `mapstructure:"executable"`
	LogDir             string `mapstructure:"log_dir"`
	DataDir            string `mapstructure:"data_dir"`
	SuspendFlagsDir    string `mapstructure:"suspend_flags_dir"`
	AdminLogsDir       string `mapstructure:"admin_logs_dir"`
	BaseConfig         string `mapstructure:"base_config"`
	AltDagsDir         string `mapstructure:"alt_dags_dir"`
	DAGRunsDir         string `mapstructure:"dag_runs_dir"`
	QueueDir           string `mapstructure:"queue_dir"`
	ProcDir            string `mapstructure:"proc_dir"`
	ServiceRegistryDir string `mapstructure:"service_registry_dir"`
	UsersDir           string `mapstructure:"users_dir"`
	APIKeysDir         string `mapstructure:"api_keys_dir"`
	WebhooksDir        string `mapstructure:"webhooks_dir"`
	SessionsDir        string `mapstructure:"sessions_dir"`
}

// -----------------------------------------------------------------------------
// UI Configuration
// -----------------------------------------------------------------------------

// UIDef configures the user interface.
type UIDef struct {
	LogEncodingCharset    string      `mapstructure:"log_encoding_charset"`
	NavbarColor           string      `mapstructure:"navbar_color"`
	NavbarTitle           string      `mapstructure:"navbar_title"`
	MaxDashboardPageLimit int         `mapstructure:"max_dashboard_page_limit"`
	DAGs                  *DAGListDef `mapstructure:"dags"`
}

// DAGListDef configures the DAGs list page.
type DAGListDef struct {
	SortField string `mapstructure:"sort_field"`
	SortOrder string `mapstructure:"sort_order"`
}

// -----------------------------------------------------------------------------
// Peer Configuration
// -----------------------------------------------------------------------------

// PeerDef configures TLS for peer gRPC connections.
type PeerDef struct {
	CertFile      string `mapstructure:"cert_file"`
	KeyFile       string `mapstructure:"key_file"`
	ClientCaFile  string `mapstructure:"client_ca_file"`
	SkipTLSVerify bool   `mapstructure:"skip_tls_verify"`
	Insecure      bool   `mapstructure:"insecure"`       // Use h2c instead of TLS
	MaxRetries    int    `mapstructure:"max_retries"`    // Default: 10
	RetryInterval string `mapstructure:"retry_interval"` // Default: 1s
}

// RemoteNodeDef configures a remote node connection.
type RemoteNodeDef struct {
	Name              string `mapstructure:"name"`
	APIBaseURL        string `mapstructure:"api_base_url"`
	IsBasicAuth       bool   `mapstructure:"is_basic_auth"`
	BasicAuthUsername string `mapstructure:"basic_auth_username"`
	BasicAuthPassword string `mapstructure:"basic_auth_password"`
	IsAuthToken       bool   `mapstructure:"is_auth_token"`
	AuthToken         string `mapstructure:"auth_token"`
	SkipTLSVerify     bool   `mapstructure:"skip_tls_verify"`
}

// -----------------------------------------------------------------------------
// Service Configuration
// -----------------------------------------------------------------------------

// CoordinatorDef configures the coordinator service.
type CoordinatorDef struct {
	Enabled   *bool  `mapstructure:"enabled"` // Default: true
	Host      string `mapstructure:"host"`
	Advertise string `mapstructure:"advertise"` // Auto-detected if empty
	Port      int    `mapstructure:"port"`
}

// WorkerDef configures the worker.
type WorkerDef struct {
	ID            string `mapstructure:"id"`
	MaxActiveRuns int    `mapstructure:"max_active_runs"`
	// Labels accepts either a string "key=value,key2=value2,..." or map[string]string.
	// When string, parsed as comma-separated key=value pairs.
	Labels any `mapstructure:"labels"`
	// Coordinators accepts either a single string URL or []string of URLs.
	// When string, used as single coordinator address.
	Coordinators any              `mapstructure:"coordinators"`
	PostgresPool *PostgresPoolDef `mapstructure:"postgres_pool"`
}

// PostgresPoolDef configures PostgreSQL connection pooling.
// Lifetime fields are specified in seconds.
type PostgresPoolDef struct {
	MaxOpenConns    int `mapstructure:"max_open_conns"`     // Maximum open connections (default: 25)
	MaxIdleConns    int `mapstructure:"max_idle_conns"`     // Maximum idle connections (default: 5)
	ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`  // Maximum connection lifetime in seconds (default: 300)
	ConnMaxIdleTime int `mapstructure:"conn_max_idle_time"` // Maximum idle time in seconds (default: 60)
}

// SchedulerDef configures the scheduler.
type SchedulerDef struct {
	Port                    int    `mapstructure:"port"`
	LockStaleThreshold      string `mapstructure:"lock_stale_threshold"`      // Default: 30s
	LockRetryInterval       string `mapstructure:"lock_retry_interval"`       // Default: 5s
	ZombieDetectionInterval string `mapstructure:"zombie_detection_interval"` // Default: 45s, 0 to disable
}

// QueueConfigDef configures global queue settings.
type QueueConfigDef struct {
	Enabled bool       `mapstructure:"enabled"`
	Config  []QueueDef `mapstructure:"config"`
}

// QueueDef configures an individual queue.
type QueueDef struct {
	Name           string `mapstructure:"name"`
	MaxActiveRuns  *int   `mapstructure:"max_active_runs"` // Deprecated: use MaxConcurrency
	MaxConcurrency int    `mapstructure:"max_concurrency"`
}

// -----------------------------------------------------------------------------
// Feature Configuration
// -----------------------------------------------------------------------------

// MonitoringDef configures system monitoring.
type MonitoringDef struct {
	Retention string `mapstructure:"retention"` // Default: 24h
	Interval  string `mapstructure:"interval"`  // Default: 5s
}

// TerminalDef configures the web-based terminal feature.
type TerminalDef struct {
	Enabled *bool `mapstructure:"enabled"` // Default: false
}

// AuditDef configures the audit logging feature.
type AuditDef struct {
	Enabled       *bool `mapstructure:"enabled"`        // Default: true
	RetentionDays *int  `mapstructure:"retention_days"` // Default: 7
}

// SessionDef configures agent session storage.
type SessionDef struct {
	MaxPerUser *int `mapstructure:"max_per_user"` // Default: 100; 0 = unlimited
}

// -----------------------------------------------------------------------------
// Git Sync Configuration
// -----------------------------------------------------------------------------

// GitSyncDef configures Git synchronization.
type GitSyncDef struct {
	Enabled     *bool               `mapstructure:"enabled"` // Default: false
	Repository  string              `mapstructure:"repository"`
	Branch      string              `mapstructure:"branch"` // Default: main
	Path        string              `mapstructure:"path"`   // Subdirectory, empty for root
	Auth        *GitSyncAuthDef     `mapstructure:"auth"`
	AutoSync    *GitSyncAutoSyncDef `mapstructure:"auto_sync"`
	PushEnabled *bool               `mapstructure:"push_enabled"` // Default: true
	Commit      *GitSyncCommitDef   `mapstructure:"commit"`
}

// GitSyncAuthDef configures Git authentication.
type GitSyncAuthDef struct {
	Type          string `mapstructure:"type"` // "token" or "ssh", default: token
	Token         string `mapstructure:"token"`
	SSHKeyPath    string `mapstructure:"ssh_key_path"`
	SSHPassphrase string `mapstructure:"ssh_passphrase"`
}

// GitSyncAutoSyncDef configures automatic synchronization.
type GitSyncAutoSyncDef struct {
	Enabled   *bool `mapstructure:"enabled"`    // Default: false
	OnStartup *bool `mapstructure:"on_startup"` // Default: true
	Interval  int   `mapstructure:"interval"`   // Seconds, default: 300
}

// GitSyncCommitDef configures Git commit metadata.
type GitSyncCommitDef struct {
	AuthorName  string `mapstructure:"author_name"`  // Default: Dagu
	AuthorEmail string `mapstructure:"author_email"` // Default: dagu@localhost
}

// -----------------------------------------------------------------------------
// Tunnel Configuration
// -----------------------------------------------------------------------------

// TunnelDef configures tunnel services.
type TunnelDef struct {
	Enabled       *bool               `mapstructure:"enabled"` // Default: false
	Tailscale     *TailscaleTunnelDef `mapstructure:"tailscale"`
	AllowTerminal *bool               `mapstructure:"allow_terminal"` // Default: false
	AllowedIPs    []string            `mapstructure:"allowed_ips"`    // Empty = allow all
	RateLimiting  *TunnelRateLimitDef `mapstructure:"rate_limiting"`
}

// TailscaleTunnelDef configures Tailscale tunnel settings.
type TailscaleTunnelDef struct {
	AuthKey  string `mapstructure:"auth_key"`
	Hostname string `mapstructure:"hostname"`  // Default: "dagu"
	Funnel   *bool  `mapstructure:"funnel"`    // Public internet access
	HTTPS    *bool  `mapstructure:"https"`     // HTTPS for tailnet-only access
	StateDir string `mapstructure:"state_dir"` // Default: $DAGU_HOME/tailscale
}

// TunnelRateLimitDef configures rate limiting for tunnel auth endpoints.
type TunnelRateLimitDef struct {
	Enabled              *bool `mapstructure:"enabled"`
	LoginAttempts        int   `mapstructure:"login_attempts"`         // Default: 5
	WindowSeconds        int   `mapstructure:"window_seconds"`         // Default: 300
	BlockDurationSeconds int   `mapstructure:"block_duration_seconds"` // Default: 900
}
