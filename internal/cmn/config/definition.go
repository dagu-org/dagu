package config

// Definition holds the overall configuration for the application.
// Fields are organized into logical groups for clarity.
type Definition struct {
	// Server settings
	Host        string  `mapstructure:"host"`
	Port        int     `mapstructure:"port"`
	BasePath    string  `mapstructure:"basePath"`
	APIBasePath string  `mapstructure:"apiBasePath"`
	APIBaseURL  string  `mapstructure:"apiBaseURL"` // Deprecated: use APIBasePath
	Headless    *bool   `mapstructure:"headless"`
	TLS         *TLSDef `mapstructure:"tls"`

	// Core settings
	Debug        bool   `mapstructure:"debug"`
	DefaultShell string `mapstructure:"defaultShell"`
	LogFormat    string `mapstructure:"logFormat"` // "json" or "text"
	TZ           string `mapstructure:"tz"`

	// Authentication
	Auth              *AuthDef `mapstructure:"auth"`
	IsBasicAuth       bool     `mapstructure:"isBasicAuth"`
	BasicAuthUsername string   `mapstructure:"basicAuthUsername"`
	BasicAuthPassword string   `mapstructure:"basicAuthPassword"`
	IsAuthToken       bool     `mapstructure:"isAuthToken"`
	AuthToken         string   `mapstructure:"authToken"`

	// Permissions
	PermissionWriteDAGs *bool          `mapstructure:"permissionWriteDAGs"`
	PermissionRunDAGs   *bool          `mapstructure:"permissionRunDAGs"`
	Permissions         PermissionsDef `mapstructure:"permissions"`

	// Paths (legacy flat fields)
	DAGs            string `mapstructure:"dags"` // Deprecated
	DAGsDir         string `mapstructure:"dagsDir"`
	Executable      string `mapstructure:"executable"`
	LogDir          string `mapstructure:"logDir"`
	DataDir         string `mapstructure:"dataDir"`
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`
	AdminLogsDir    string `mapstructure:"adminLogsDir"`
	BaseConfig      string `mapstructure:"baseConfig"`

	// Paths (structured)
	Paths *PathsDef `mapstructure:"paths"`

	// UI settings (legacy flat fields)
	LogEncodingCharset    string `mapstructure:"logEncodingCharset"`
	NavbarColor           string `mapstructure:"navbarColor"`
	NavbarTitle           string `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int    `mapstructure:"maxDashboardPageLimit"`
	LatestStatusToday     *bool  `mapstructure:"latestStatusToday"`

	// UI settings (structured)
	UI *UIDef `mapstructure:"ui"`

	// Peer connections
	Peer PeerDef `mapstructure:"peer"`

	// Remote nodes
	RemoteNodes []RemoteNodeDef `mapstructure:"remoteNodes"`

	// Services
	Coordinator *CoordinatorDef `mapstructure:"coordinator"`
	Worker      *WorkerDef      `mapstructure:"worker"`
	Scheduler   *SchedulerDef   `mapstructure:"scheduler"`
	Queues      *QueueConfigDef `mapstructure:"queues"`

	// Execution
	DefaultExecutionMode string `mapstructure:"defaultExecutionMode"`

	// Features
	Monitoring *MonitoringDef `mapstructure:"monitoring"`
	Metrics    *string        `mapstructure:"metrics"` // "public" or "private"
	Cache      *string        `mapstructure:"cache"`   // "low", "normal", or "high"
	Terminal   *TerminalDef   `mapstructure:"terminal"`
	Audit      *AuditDef      `mapstructure:"audit"`
	GitSync    *GitSyncDef    `mapstructure:"gitSync"`
	Tunnel     *TunnelDef     `mapstructure:"tunnel"`
}

// -----------------------------------------------------------------------------
// Server Configuration
// -----------------------------------------------------------------------------

// TLSDef configures TLS/SSL encryption.
type TLSDef struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
	CAFile   string `mapstructure:"caFile"`
}

// -----------------------------------------------------------------------------
// Authentication Configuration
// -----------------------------------------------------------------------------

// AuthDef configures authentication for the application.
type AuthDef struct {
	Mode    *string         `mapstructure:"mode"` // "none", "builtin", or "oidc"
	Basic   *AuthBasicDef   `mapstructure:"basic"`
	Token   *AuthTokenDef   `mapstructure:"token"`
	OIDC    *AuthOIDCDef    `mapstructure:"oidc"`
	Builtin *AuthBuiltinDef `mapstructure:"builtin"`
}

// AuthBasicDef configures basic authentication credentials.
type AuthBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthTokenDef configures token-based authentication.
type AuthTokenDef struct {
	Value string `mapstructure:"value"`
}

// AuthBuiltinDef configures builtin authentication with RBAC.
type AuthBuiltinDef struct {
	Admin *AdminConfigDef `mapstructure:"admin"`
	Token *TokenConfigDef `mapstructure:"token"`
}

// AdminConfigDef configures the initial admin user.
type AdminConfigDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TokenConfigDef configures JWT token settings.
type TokenConfigDef struct {
	Secret string `mapstructure:"secret"`
	TTL    string `mapstructure:"ttl"`
}

// AuthOIDCDef configures OIDC authentication.
// Core fields are used by both standalone and builtin auth modes.
// Builtin-specific fields are only used when auth.mode=builtin.
type AuthOIDCDef struct {
	// ClientID is the OAuth client identifier (Go naming: ID not Id).
	// mapstructure tag uses lowercase "clientId" for YAML compatibility.
	ClientID     string `mapstructure:"clientId"`
	ClientSecret string `mapstructure:"clientSecret"`
	// ClientURL is the application callback URL (Go naming: URL not Url).
	// mapstructure tag uses lowercase "clientUrl" for YAML compatibility.
	ClientURL      string              `mapstructure:"clientUrl"`
	Issuer         string              `mapstructure:"issuer"`
	Scopes         []string            `mapstructure:"scopes"`
	Whitelist      []string            `mapstructure:"whitelist"`
	AutoSignup     *bool               `mapstructure:"autoSignup"` // Default: true (builtin mode only)
	AllowedDomains []string            `mapstructure:"allowedDomains"`
	ButtonLabel    string              `mapstructure:"buttonLabel"`
	RoleMapping    *OIDCRoleMappingDef `mapstructure:"roleMapping"`
}

// OIDCRoleMappingDef maps OIDC claims to Dagu roles.
type OIDCRoleMappingDef struct {
	DefaultRole         string            `mapstructure:"defaultRole"`         // Default: "viewer"
	GroupsClaim         string            `mapstructure:"groupsClaim"`         // Default: "groups"
	GroupMappings       map[string]string `mapstructure:"groupMappings"`       // IdP group -> Dagu role
	RoleAttributePath   string            `mapstructure:"roleAttributePath"`   // jq expression for role extraction
	RoleAttributeStrict *bool             `mapstructure:"roleAttributeStrict"` // Deny login if no valid role found
	SkipOrgRoleSync     *bool             `mapstructure:"skipOrgRoleSync"`     // Only assign roles on first login
}

// PermissionsDef configures UI and API permissions.
type PermissionsDef struct {
	WriteDAGs *bool `mapstructure:"writeDAGs"`
	RunDAGs   *bool `mapstructure:"runDAGs"`
}

// -----------------------------------------------------------------------------
// Path Configuration
// -----------------------------------------------------------------------------

// PathsDef configures file system paths.
type PathsDef struct {
	DAGsDir            string `mapstructure:"dagsDir"`
	Executable         string `mapstructure:"executable"`
	LogDir             string `mapstructure:"logDir"`
	DataDir            string `mapstructure:"dataDir"`
	SuspendFlagsDir    string `mapstructure:"suspendFlagsDir"`
	AdminLogsDir       string `mapstructure:"adminLogsDir"`
	BaseConfig         string `mapstructure:"baseConfig"`
	DAGRunsDir         string `mapstructure:"dagRunsDir"`
	QueueDir           string `mapstructure:"queueDir"`
	ProcDir            string `mapstructure:"procDir"`
	ServiceRegistryDir string `mapstructure:"serviceRegistryDir"`
	UsersDir           string `mapstructure:"usersDir"`
	APIKeysDir         string `mapstructure:"apiKeysDir"`
	WebhooksDir        string `mapstructure:"webhooksDir"`
	SessionsDir        string `mapstructure:"sessionsDir"`
}

// -----------------------------------------------------------------------------
// UI Configuration
// -----------------------------------------------------------------------------

// UIDef configures the user interface.
type UIDef struct {
	LogEncodingCharset    string      `mapstructure:"logEncodingCharset"`
	NavbarColor           string      `mapstructure:"navbarColor"`
	NavbarTitle           string      `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int         `mapstructure:"maxDashboardPageLimit"`
	DAGs                  *DAGListDef `mapstructure:"dags"`
}

// DAGListDef configures the DAGs list page.
type DAGListDef struct {
	SortField string `mapstructure:"sortField"`
	SortOrder string `mapstructure:"sortOrder"`
}

// -----------------------------------------------------------------------------
// Peer Configuration
// -----------------------------------------------------------------------------

// PeerDef configures TLS for peer gRPC connections.
type PeerDef struct {
	CertFile      string `mapstructure:"certFile"`
	KeyFile       string `mapstructure:"keyFile"`
	ClientCaFile  string `mapstructure:"clientCaFile"`
	SkipTLSVerify bool   `mapstructure:"skipTlsVerify"`
	Insecure      bool   `mapstructure:"insecure"`      // Use h2c instead of TLS
	MaxRetries    int    `mapstructure:"maxRetries"`    // Default: 10
	RetryInterval string `mapstructure:"retryInterval"` // Default: 1s
}

// RemoteNodeDef configures a remote node connection.
type RemoteNodeDef struct {
	Name              string `mapstructure:"name"`
	APIBaseURL        string `mapstructure:"apiBaseURL"`
	IsBasicAuth       bool   `mapstructure:"isBasicAuth"`
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`
	IsAuthToken       bool   `mapstructure:"isAuthToken"`
	AuthToken         string `mapstructure:"authToken"`
	SkipTLSVerify     bool   `mapstructure:"skipTLSVerify"`
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
	MaxActiveRuns int    `mapstructure:"maxActiveRuns"`
	// Labels accepts either a string "key=value,key2=value2,..." or map[string]string.
	// When string, parsed as comma-separated key=value pairs.
	Labels any `mapstructure:"labels"`
	// Coordinators accepts either a single string URL or []string of URLs.
	// When string, used as single coordinator address.
	Coordinators any              `mapstructure:"coordinators"`
	PostgresPool *PostgresPoolDef `mapstructure:"postgresPool"`
}

// PostgresPoolDef configures PostgreSQL connection pooling.
// Lifetime fields are specified in seconds.
type PostgresPoolDef struct {
	MaxOpenConns    int `mapstructure:"maxOpenConns"`    // Maximum open connections (default: 25)
	MaxIdleConns    int `mapstructure:"maxIdleConns"`    // Maximum idle connections (default: 5)
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"` // Maximum connection lifetime in seconds (default: 300)
	ConnMaxIdleTime int `mapstructure:"connMaxIdleTime"` // Maximum idle time in seconds (default: 60)
}

// SchedulerDef configures the scheduler.
type SchedulerDef struct {
	Port                    int    `mapstructure:"port"`
	LockStaleThreshold      string `mapstructure:"lockStaleThreshold"`      // Default: 30s
	LockRetryInterval       string `mapstructure:"lockRetryInterval"`       // Default: 5s
	ZombieDetectionInterval string `mapstructure:"zombieDetectionInterval"` // Default: 45s, 0 to disable
}

// QueueConfigDef configures global queue settings.
type QueueConfigDef struct {
	Enabled bool       `mapstructure:"enabled"`
	Config  []QueueDef `mapstructure:"config"`
}

// QueueDef configures an individual queue.
type QueueDef struct {
	Name           string `mapstructure:"name"`
	MaxActiveRuns  *int   `mapstructure:"maxActiveRuns"` // Deprecated: use MaxConcurrency
	MaxConcurrency int    `mapstructure:"maxConcurrency"`
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
	Enabled       *bool `mapstructure:"enabled"`       // Default: true
	RetentionDays *int  `mapstructure:"retentionDays"` // Default: 7
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
	AutoSync    *GitSyncAutoSyncDef `mapstructure:"autoSync"`
	PushEnabled *bool               `mapstructure:"pushEnabled"` // Default: true
	Commit      *GitSyncCommitDef   `mapstructure:"commit"`
}

// GitSyncAuthDef configures Git authentication.
type GitSyncAuthDef struct {
	Type          string `mapstructure:"type"` // "token" or "ssh", default: token
	Token         string `mapstructure:"token"`
	SSHKeyPath    string `mapstructure:"sshKeyPath"`
	SSHPassphrase string `mapstructure:"sshPassphrase"`
}

// GitSyncAutoSyncDef configures automatic synchronization.
type GitSyncAutoSyncDef struct {
	Enabled   *bool `mapstructure:"enabled"`   // Default: false
	OnStartup *bool `mapstructure:"onStartup"` // Default: true
	Interval  int   `mapstructure:"interval"`  // Seconds, default: 300
}

// GitSyncCommitDef configures Git commit metadata.
type GitSyncCommitDef struct {
	AuthorName  string `mapstructure:"authorName"`  // Default: Dagu
	AuthorEmail string `mapstructure:"authorEmail"` // Default: dagu@localhost
}

// -----------------------------------------------------------------------------
// Tunnel Configuration
// -----------------------------------------------------------------------------

// TunnelDef configures tunnel services.
type TunnelDef struct {
	Enabled       *bool               `mapstructure:"enabled"` // Default: false
	Tailscale     *TailscaleTunnelDef `mapstructure:"tailscale"`
	AllowTerminal *bool               `mapstructure:"allowTerminal"` // Default: false
	AllowedIPs    []string            `mapstructure:"allowedIPs"`    // Empty = allow all
	RateLimiting  *TunnelRateLimitDef `mapstructure:"rateLimiting"`
}

// TailscaleTunnelDef configures Tailscale tunnel settings.
type TailscaleTunnelDef struct {
	AuthKey  string `mapstructure:"authKey"`
	Hostname string `mapstructure:"hostname"` // Default: "dagu"
	Funnel   *bool  `mapstructure:"funnel"`   // Public internet access
	HTTPS    *bool  `mapstructure:"https"`    // HTTPS for tailnet-only access
	StateDir string `mapstructure:"stateDir"` // Default: $DAGU_HOME/tailscale
}

// TunnelRateLimitDef configures rate limiting for tunnel auth endpoints.
type TunnelRateLimitDef struct {
	Enabled              *bool `mapstructure:"enabled"`
	LoginAttempts        int   `mapstructure:"loginAttempts"`        // Default: 5
	WindowSeconds        int   `mapstructure:"windowSeconds"`        // Default: 300
	BlockDurationSeconds int   `mapstructure:"blockDurationSeconds"` // Default: 900
}
