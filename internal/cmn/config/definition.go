package config

// Definition holds the overall configuration for the application.
type Definition struct {
	Peer                  PeerDef         `mapstructure:"peer"`
	Host                  string          `mapstructure:"host"`
	Port                  int             `mapstructure:"port"`
	PermissionWriteDAGs   *bool           `mapstructure:"permissionWriteDAGs"`
	PermissionRunDAGs     *bool           `mapstructure:"permissionRunDAGs"`
	Permissions           PermissionsDef  `mapstructure:"permissions"`
	Debug                 bool            `mapstructure:"debug"`
	BasePath              string          `mapstructure:"basePath"`
	APIBasePath           string          `mapstructure:"apiBasePath"`
	APIBaseURL            string          `mapstructure:"apiBaseURL"` // Deprecated: Use APIBasePath instead
	DefaultShell          string          `mapstructure:"defaultShell"`
	Headless              *bool           `mapstructure:"headless"`
	Auth                  *AuthDef        `mapstructure:"auth"`
	Paths                 *PathsDef       `mapstructure:"paths"`
	LogFormat             string          `mapstructure:"logFormat"` // "json" or "text"
	LatestStatusToday     *bool           `mapstructure:"latestStatusToday"`
	TZ                    string          `mapstructure:"tz"`
	UI                    *UIDef          `mapstructure:"ui"`
	RemoteNodes           []RemoteNodeDef `mapstructure:"remoteNodes"`
	TLS                   *TLSDef         `mapstructure:"tls"`
	Queues                *QueueConfigDef `mapstructure:"queues"`
	Coordinator           *CoordinatorDef `mapstructure:"coordinator"`
	Worker                *WorkerDef      `mapstructure:"worker"`
	Scheduler             *SchedulerDef   `mapstructure:"scheduler"`
	DAGs                  string          `mapstructure:"dags"` // Deprecated
	DAGsDir               string          `mapstructure:"dagsDir"`
	Executable            string          `mapstructure:"executable"`
	LogDir                string          `mapstructure:"logDir"`
	DataDir               string          `mapstructure:"dataDir"`
	SuspendFlagsDir       string          `mapstructure:"suspendFlagsDir"`
	AdminLogsDir          string          `mapstructure:"adminLogsDir"`
	BaseConfig            string          `mapstructure:"baseConfig"`
	IsBasicAuth           bool            `mapstructure:"isBasicAuth"`
	BasicAuthUsername     string          `mapstructure:"basicAuthUsername"`
	BasicAuthPassword     string          `mapstructure:"basicAuthPassword"`
	IsAuthToken           bool            `mapstructure:"isAuthToken"`
	AuthToken             string          `mapstructure:"authToken"`
	LogEncodingCharset    string          `mapstructure:"logEncodingCharset"`
	NavbarColor           string          `mapstructure:"navbarColor"`
	NavbarTitle           string          `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int             `mapstructure:"maxDashboardPageLimit"`
	Monitoring            *MonitoringDef  `mapstructure:"monitoring"`
	Metrics               *string         `mapstructure:"metrics"` // "public" or "private" (default: "private")
	Cache                 *string         `mapstructure:"cache"`   // "low", "normal", or "high" (default: "normal")
	Terminal              *TerminalDef    `mapstructure:"terminal"`
	Audit                 *AuditDef       `mapstructure:"audit"`
	GitSync               *GitSyncDef     `mapstructure:"gitSync"`
	Tunnel                *TunnelDef      `mapstructure:"tunnel"`
}

// TerminalDef configures the web-based terminal feature.
type TerminalDef struct {
	Enabled *bool `mapstructure:"enabled"` // Default: false
}

// AuditDef configures the audit logging feature.
type AuditDef struct {
	Enabled *bool `mapstructure:"enabled"` // Default: true
}

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

// AuthDef configures authentication for the application.
type AuthDef struct {
	Mode    *string         `mapstructure:"mode"` // "none", "builtin", or "oidc"
	Basic   *AuthBasicDef   `mapstructure:"basic"`
	Token   *AuthTokenDef   `mapstructure:"token"`
	OIDC    *AuthOIDCDef    `mapstructure:"oidc"`
	Builtin *AuthBuiltinDef `mapstructure:"builtin"`
}

// AuthBuiltinDef configures builtin authentication with RBAC.
type AuthBuiltinDef struct {
	Admin *AdminConfigDef `mapstructure:"admin"`
	Token *TokenConfigDef `mapstructure:"token"`
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

// AuthBasicDef configures basic authentication credentials.
type AuthBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthTokenDef configures token-based authentication.
type AuthTokenDef struct {
	Value string `mapstructure:"value"`
}

// AuthOIDCDef configures OIDC authentication.
// Core fields are used by both standalone and builtin auth modes.
// Builtin-specific fields are only used when auth.mode=builtin.
type AuthOIDCDef struct {
	ClientId       string              `mapstructure:"clientId"`
	ClientSecret   string              `mapstructure:"clientSecret"`
	ClientUrl      string              `mapstructure:"clientUrl"`
	Issuer         string              `mapstructure:"issuer"`
	Scopes         []string            `mapstructure:"scopes"`
	Whitelist      []string            `mapstructure:"whitelist"`
	AutoSignup     *bool               `mapstructure:"autoSignup"` // Default: true (builtin mode only)
	AllowedDomains []string            `mapstructure:"allowedDomains"`
	ButtonLabel    string              `mapstructure:"buttonLabel"`
	RoleMapping    *OIDCRoleMappingDef `mapstructure:"roleMapping"`
}

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
	ConversationsDir   string `mapstructure:"conversationsDir"`
}

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

// PermissionsDef configures UI and API permissions.
type PermissionsDef struct {
	WriteDAGs *bool `mapstructure:"writeDAGs"`
	RunDAGs   *bool `mapstructure:"runDAGs"`
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

// TLSDef configures TLS/SSL encryption.
type TLSDef struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
	CAFile   string `mapstructure:"caFile"`
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

// CoordinatorDef configures the coordinator service.
type CoordinatorDef struct {
	Host      string `mapstructure:"host"`
	Advertise string `mapstructure:"advertise"` // Auto-detected if empty
	Port      int    `mapstructure:"port"`
}

// MonitoringDef configures system monitoring.
type MonitoringDef struct {
	Retention string `mapstructure:"retention"` // Default: 24h
	Interval  string `mapstructure:"interval"`  // Default: 5s
}

// WorkerDef configures the worker.
type WorkerDef struct {
	ID            string           `mapstructure:"id"`
	MaxActiveRuns int              `mapstructure:"maxActiveRuns"`
	Labels        interface{}      `mapstructure:"labels"`       // String or map
	Coordinators  interface{}      `mapstructure:"coordinators"` // String or list for static discovery
	PostgresPool  *PostgresPoolDef `mapstructure:"postgresPool"`
}

// SchedulerDef configures the scheduler.
type SchedulerDef struct {
	Port                    int    `mapstructure:"port"`
	LockStaleThreshold      string `mapstructure:"lockStaleThreshold"`      // Default: 30s
	LockRetryInterval       string `mapstructure:"lockRetryInterval"`       // Default: 5s
	ZombieDetectionInterval string `mapstructure:"zombieDetectionInterval"` // Default: 45s, 0 to disable
}

// PostgresPoolDef configures PostgreSQL connection pooling.
type PostgresPoolDef struct {
	MaxOpenConns    int `mapstructure:"maxOpenConns"`    // Default: 25
	MaxIdleConns    int `mapstructure:"maxIdleConns"`    // Default: 5
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"` // Seconds, default: 300
	ConnMaxIdleTime int `mapstructure:"connMaxIdleTime"` // Seconds, default: 60
}

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
