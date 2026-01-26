package config

// Definition holds the overall configuration for the application.
// Each field maps to a configuration key defined in external sources (like YAML files)
type Definition struct {
	// Peer contains configuration for peer connections over gRPC.
	Peer PeerDef `mapstructure:"peer"`

	// Host defines the hostname or IP address on which the application will run.
	Host string `mapstructure:"host"`

	// Port specifies the network port for incoming connections.
	Port int `mapstructure:"port"`

	// PermissionWriteDAGs indicates if the user has permission to write DAGs.
	PermissionWriteDAGs *bool `mapstructure:"permissionWriteDAGs"`

	// PermissionRunDAGs indicates if the user has permission to run DAGs.
	PermissionRunDAGs *bool `mapstructure:"permissionRunDAGs"`

	// Permissions defines the permissions allowed in the UI and API.
	Permissions PermissionsDef `mapstructure:"permissions"`

	// Debug toggles debug mode; when true, the application may output extra logs and error details.
	Debug bool `mapstructure:"debug"`

	// BasePath is the root URL path from which the application is served.
	// This is useful when hosting the app behind a reverse proxy under a subpath.
	BasePath string `mapstructure:"basePath"`

	// APIBasePath sets the base path for all API endpoints provided by the application.
	APIBasePath string `mapstructure:"apiBasePath"`

	// APIBaseURL is a deprecated field that previously specified the full base URL for the API.
	// Use APIBasePath instead.
	APIBaseURL string `mapstructure:"apiBaseURL"`

	// DefaultShell specifies the default shell to use for command execution.
	// If not provided, platform-specific defaults are used (PowerShell on Windows, $SHELL on Unix).
	DefaultShell string `mapstructure:"defaultShell"`

	// Headless determines if the application should run without a graphical user interface.
	// Useful for automated or headless server environments.
	Headless *bool `mapstructure:"headless"`

	// Auth contains authentication settings (such as credentials or tokens) needed to secure the application.
	Auth *AuthDef `mapstructure:"auth"`

	// Paths holds various filesystem path configurations used throughout the application.
	Paths *PathsDef `mapstructure:"paths"`

	// LogFormat defines the output format for log messages (e.g., JSON, plain text).
	// Available options: "json", "text"
	LogFormat string `mapstructure:"logFormat"`

	// LatestStatusToday indicates whether the application should display only the most recent status for the current day.
	LatestStatusToday *bool `mapstructure:"latestStatusToday"`

	// TZ represents the timezone setting for the application (for example, "UTC" or "America/New_York").
	TZ string `mapstructure:"tz"`

	// UI contains settings specific to the application's user interface.
	UI *UIDef `mapstructure:"ui"`

	// RemoteNodes holds a list of configurations for connecting to remote nodes.
	// This enables the management of DAGs on external servers.
	RemoteNodes []RemoteNodeDef `mapstructure:"remoteNodes"`

	// TLS contains configuration details for enabling TLS/SSL encryption,
	// such as certificate and key file paths.
	TLS *TLSDef `mapstructure:"tls"`

	// Queues contains global queue configuration settings.
	Queues *QueueConfigDef `mapstructure:"queues"`

	// Coordinator contains configuration for the coordinator server.
	Coordinator *CoordinatorDef `mapstructure:"coordinator"`

	// Worker contains configuration for the worker.
	Worker *WorkerDef `mapstructure:"worker"`

	// Scheduler contains configuration for the scheduler.
	Scheduler *SchedulerDef `mapstructure:"scheduler"`

	// DAGs is a field that was previously used to configure the directory for DAG files.
	DAGs string `mapstructure:"dags"`

	// DAGsDir specifies the directory where DAG files are stored.
	DAGsDir string `mapstructure:"dagsDir"`

	// Executable indicates the path to the executable used for running DAG tasks.
	Executable string `mapstructure:"executable"`

	// LogDir defines the directory where log files are saved.
	LogDir string `mapstructure:"logDir"`

	// DataDir specifies the directory for storing application data, such as history or state.
	DataDir string `mapstructure:"dataDir"`

	// SuspendFlagsDir sets the directory used for storing flags that indicate a DAG is suspended.
	SuspendFlagsDir string `mapstructure:"suspendFlagsDir"`

	// AdminLogsDir indicates the directory for storing administrative logs.
	AdminLogsDir string `mapstructure:"adminLogsDir"`

	// BaseConfig provides the path to a base configuration file shared across DAGs.
	BaseConfig string `mapstructure:"baseConfig"`

	// IsBasicAuth indicates whether basic authentication is enabled.
	IsBasicAuth bool `mapstructure:"isBasicAuth"`

	// BasicAuthUsername holds the username for basic authentication.
	BasicAuthUsername string `mapstructure:"basicAuthUsername"`

	// BasicAuthPassword holds the password for basic authentication.
	BasicAuthPassword string `mapstructure:"basicAuthPassword"`

	// IsAuthToken indicates whether token-based authentication is enabled.
	IsAuthToken bool `mapstructure:"isAuthToken"`

	// AuthToken holds the token value for API authentication.
	AuthToken string `mapstructure:"authToken"`

	// LogEncodingCharset defines the character encoding used in log files.
	LogEncodingCharset string `mapstructure:"logEncodingCharset"`

	// NavbarColor sets the color of the navigation bar in the application's UI.
	NavbarColor string `mapstructure:"navbarColor"`

	// NavbarTitle specifies the title text displayed in the navigation bar of the UI.
	NavbarTitle string `mapstructure:"navbarTitle"`

	// MaxDashboardPageLimit limits the number of dashboard pages that can be shown in the UI.
	MaxDashboardPageLimit int `mapstructure:"maxDashboardPageLimit"`

	// Monitoring contains configuration for system monitoring.
	Monitoring *MonitoringDef `mapstructure:"monitoring"`

	// Metrics controls access to the /api/v2/metrics endpoint.
	// Valid values: "public", "private" (default: "private")
	Metrics *string `mapstructure:"metrics"`

	// Cache specifies the cache mode preset.
	// Valid values: "low", "normal", "high" (default: "normal")
	Cache *string `mapstructure:"cache"`

	// Terminal contains configuration for the web-based terminal feature.
	Terminal *TerminalDef `mapstructure:"terminal"`

	// Audit contains configuration for the audit logging feature.
	Audit *AuditDef `mapstructure:"audit"`

	// GitSync contains configuration for Git synchronization.
	GitSync *GitSyncDef `mapstructure:"gitSync"`

	// Tunnel contains configuration for tunnel services (Cloudflare/Tailscale).
	Tunnel *TunnelDef `mapstructure:"tunnel"`
}

// TerminalDef represents the terminal configuration.
type TerminalDef struct {
	// Enabled determines if the terminal feature is available.
	// Default: false
	// Env: DAGU_TERMINAL_ENABLED
	Enabled *bool `mapstructure:"enabled"`
}

// AuditDef represents the audit logging configuration.
type AuditDef struct {
	// Enabled determines if audit logging is active.
	// Default: true
	// Env: DAGU_AUDIT_ENABLED
	Enabled *bool `mapstructure:"enabled"`
}

// PeerDef holds the certificate and TLS configuration for peer connections over gRPC.
type PeerDef struct {
	// CertFile is the path to the server's TLS certificate file.
	CertFile string `mapstructure:"certFile"`

	// KeyFile is the path to the server's TLS key file.
	KeyFile string `mapstructure:"keyFile"`

	// ClientCaFile is the path to the CA certificate file used for client verification.
	ClientCaFile string `mapstructure:"clientCaFile"`

	// SkipTLSVerify indicates whether to skip TLS certificate verification.
	SkipTLSVerify bool `mapstructure:"skipTlsVerify"`

	// Insecure indicates whether to use insecure connection (h2c) instead of TLS.
	Insecure bool `mapstructure:"insecure"`

	// MaxRetries is the maximum number of retry attempts for coordinator connections.
	// Default: 10
	MaxRetries int `mapstructure:"maxRetries"`

	// RetryInterval is the base interval between retry attempts.
	// Default: 1s
	RetryInterval string `mapstructure:"retryInterval"`
}

// AuthDef holds the authentication configuration for the application.
type AuthDef struct {
	// Mode specifies the authentication mode: "none", "builtin", or "oidc"
	Mode    *string         `mapstructure:"mode"`
	Basic   *AuthBasicDef   `mapstructure:"basic"`
	Token   *AuthTokenDef   `mapstructure:"token"`
	OIDC    *AuthOIDCDef    `mapstructure:"oidc"`
	Builtin *AuthBuiltinDef `mapstructure:"builtin"`
}

// AuthBuiltinDef represents the builtin authentication configuration
type AuthBuiltinDef struct {
	Admin *AdminConfigDef `mapstructure:"admin"`
	Token *TokenConfigDef `mapstructure:"token"`
}

// OIDCRoleMappingDef defines how OIDC claims are mapped to Dagu roles
type OIDCRoleMappingDef struct {
	// DefaultRole is the role assigned to new OIDC users when no mapping matches (default: "viewer")
	DefaultRole string `mapstructure:"defaultRole"`
	// GroupsClaim specifies the claim name containing groups (default: "groups")
	// Common values: "groups", "roles", "cognito:groups", "realm_access.roles"
	GroupsClaim string `mapstructure:"groupsClaim"`
	// GroupMappings maps IdP group names to Dagu roles
	// Example: {"admins": "admin", "developers": "manager", "ops": "operator"}
	GroupMappings map[string]string `mapstructure:"groupMappings"`
	// RoleAttributePath is a jq expression to extract role from claims (advanced)
	// Example: 'if (.groups | contains(["admins"])) then "admin" else "viewer" end'
	RoleAttributePath string `mapstructure:"roleAttributePath"`
	// RoleAttributeStrict denies login when no valid role is found (default: false)
	RoleAttributeStrict *bool `mapstructure:"roleAttributeStrict"`
	// SkipOrgRoleSync skips role sync on subsequent logins (default: false)
	// When true, roles are only assigned on first login
	SkipOrgRoleSync *bool `mapstructure:"skipOrgRoleSync"`
}

// AdminConfigDef represents the initial admin user configuration
type AdminConfigDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TokenConfigDef represents the JWT token configuration
type TokenConfigDef struct {
	Secret string `mapstructure:"secret"`
	TTL    string `mapstructure:"ttl"`
}

// AuthBasicDef represents the basic authentication configuration
type AuthBasicDef struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthTokenDef represents the authentication token configuration
type AuthTokenDef struct {
	Value string `mapstructure:"value"`
}

// AuthOIDCDef represents the OIDC authentication configuration.
// Core fields are used by both standalone OIDC mode and builtin auth mode with OIDC.
// Builtin-specific fields are only used when auth.mode=builtin.
// OIDC is automatically enabled under builtin mode when all required fields
// (clientId, clientSecret, clientUrl, issuer) are configured.
type AuthOIDCDef struct {
	// Core OIDC fields (used by both standalone and builtin modes)
	ClientId     string   `mapstructure:"clientId"`
	ClientSecret string   `mapstructure:"clientSecret"`
	ClientUrl    string   `mapstructure:"clientUrl"`
	Issuer       string   `mapstructure:"issuer"`
	Scopes       []string `mapstructure:"scopes"`
	Whitelist    []string `mapstructure:"whitelist"`

	// Builtin-specific fields (only used when auth.mode=builtin)
	AutoSignup     *bool               `mapstructure:"autoSignup"`     // Auto-create users on first login (default: true)
	AllowedDomains []string            `mapstructure:"allowedDomains"` // Email domain whitelist
	ButtonLabel    string              `mapstructure:"buttonLabel"`    // Login button text
	RoleMapping    *OIDCRoleMappingDef `mapstructure:"roleMapping"`    // Role mapping configuration
}

// PathsDef represents the file system paths configuration.
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
}

// UIDef holds the user interface configuration settings.
type UIDef struct {
	LogEncodingCharset    string      `mapstructure:"logEncodingCharset"`
	NavbarColor           string      `mapstructure:"navbarColor"`
	NavbarTitle           string      `mapstructure:"navbarTitle"`
	MaxDashboardPageLimit int         `mapstructure:"maxDashboardPageLimit"`
	DAGs                  *DAGListDef `mapstructure:"dags"`
}

// DAGListDef holds the DAGs page configuration settings.
type DAGListDef struct {
	SortField string `mapstructure:"sortField"`
	SortOrder string `mapstructure:"sortOrder"`
}

// PermissionsDef holds the permissions configuration for the application.
// It defines what actions are allowed in the UI, such as writing DAGs.
type PermissionsDef struct {
	WriteDAGs *bool `mapstructure:"writeDAGs"`
	RunDAGs   *bool `mapstructure:"runDAGs"`
}

// RemoteNodeDef represents a configuration for connecting to a remote node.
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

// TLSDef represents TLS configuration
type TLSDef struct {
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
	CAFile   string `mapstructure:"caFile"`
}

// QueueConfigDef represents the global queue configuration
type QueueConfigDef struct {
	Enabled bool       `mapstructure:"enabled"`
	Config  []QueueDef `mapstructure:"config"`
}

// QueueDef represents individual queue configuration
type QueueDef struct {
	Name string `mapstructure:"name"`
	// Deprecated: use maxConcurrency
	MaxActiveRuns  *int `mapstructure:"maxActiveRuns"`
	MaxConcurrency int  `mapstructure:"maxConcurrency"`
}

// CoordinatorDef holds the configuration for the coordinator service.
type CoordinatorDef struct {
	// Host is the bind address for the coordinator gRPC server.
	Host string `mapstructure:"host"`

	// Advertise is the address to advertise in the service registry.
	// If empty, the hostname will be auto-detected.
	Advertise string `mapstructure:"advertise"`

	// Port is the port number for the coordinator service.
	Port int `mapstructure:"port"`
}

// MonitoringDef holds the configuration for system monitoring.
type MonitoringDef struct {
	// Retention specifies how long to keep system resource history.
	// Default is 24h.
	Retention string `mapstructure:"retention"`
	// Interval specifies how often to collect resource metrics.
	// Default is 5s.
	Interval string `mapstructure:"interval"`
}

// WorkerDef holds the configuration for the worker.
type WorkerDef struct {
	// ID is the unique identifier for the worker instance.
	ID string `mapstructure:"id"`

	// MaxActiveRuns is the maximum number of active runs for the worker.
	MaxActiveRuns int `mapstructure:"maxActiveRuns"`

	// Labels are the worker labels for capability matching.
	// Can be either a string (key1=value1,key2=value2) or a map in YAML.
	Labels interface{} `mapstructure:"labels"`

	// Coordinators is a list of coordinator addresses for static service discovery.
	// Can be either a string (host1:port1,host2:port2) or a list in YAML.
	// When specified, the worker will connect directly to these coordinators
	// instead of using the file-based service registry.
	Coordinators interface{} `mapstructure:"coordinators"`

	// PostgresPool holds connection pool settings for shared-nothing mode.
	PostgresPool *PostgresPoolDef `mapstructure:"postgresPool"`
}

// SchedulerDef holds the configuration for the scheduler.
type SchedulerDef struct {
	// Port is the port number for the health check server.
	Port int `mapstructure:"port"`

	// LockStaleThreshold is the time after which a scheduler lock is considered stale.
	// Default is 30 seconds.
	LockStaleThreshold string `mapstructure:"lockStaleThreshold"`

	// LockRetryInterval is the interval between lock acquisition attempts.
	// Default is 5 second.
	LockRetryInterval string `mapstructure:"lockRetryInterval"`

	// ZombieDetectionInterval is the interval between checks for zombie DAG runs.
	// Default is 45 seconds. Set to 0 to disable.
	ZombieDetectionInterval string `mapstructure:"zombieDetectionInterval"`
}

// PostgresPoolDef holds the definition for PostgreSQL connection pool configuration.
// Used in shared-nothing worker mode to prevent connection exhaustion.
type PostgresPoolDef struct {
	// MaxOpenConns is the maximum total open connections across ALL PostgreSQL DSNs.
	// Default: 25
	MaxOpenConns int `mapstructure:"maxOpenConns"`

	// MaxIdleConns is the maximum number of idle connections per DSN.
	// Default: 5
	MaxIdleConns int `mapstructure:"maxIdleConns"`

	// ConnMaxLifetime is the maximum lifetime of a connection in seconds.
	// Default: 300 (5 minutes)
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"`

	// ConnMaxIdleTime is the maximum idle time for a connection in seconds.
	// Default: 60 (1 minute)
	ConnMaxIdleTime int `mapstructure:"connMaxIdleTime"`
}

// GitSyncDef holds the definition for Git synchronization configuration.
type GitSyncDef struct {
	// Enabled indicates whether Git sync is enabled.
	// Default: false
	// Env: DAGU_GITSYNC_ENABLED
	Enabled *bool `mapstructure:"enabled"`

	// Repository is the Git repository URL.
	// Format: github.com/org/repo or https://github.com/org/repo.git
	// Env: DAGU_GITSYNC_REPOSITORY
	Repository string `mapstructure:"repository"`

	// Branch is the branch to sync with.
	// Default: main
	// Env: DAGU_GITSYNC_BRANCH
	Branch string `mapstructure:"branch"`

	// Path is the subdirectory within the repository to sync.
	// Empty string means root directory.
	// Env: DAGU_GITSYNC_PATH
	Path string `mapstructure:"path"`

	// Auth contains authentication configuration.
	Auth *GitSyncAuthDef `mapstructure:"auth"`

	// AutoSync contains auto-sync configuration.
	AutoSync *GitSyncAutoSyncDef `mapstructure:"autoSync"`

	// PushEnabled indicates whether pushing changes is allowed.
	// Default: true
	// Env: DAGU_GITSYNC_PUSH_ENABLED
	PushEnabled *bool `mapstructure:"pushEnabled"`

	// Commit contains commit configuration.
	Commit *GitSyncCommitDef `mapstructure:"commit"`
}

// GitSyncAuthDef holds authentication configuration for Git operations.
type GitSyncAuthDef struct {
	// Type is the authentication type: "token" or "ssh".
	// Default: token
	// Env: DAGU_GITSYNC_AUTH_TYPE
	Type string `mapstructure:"type"`

	// Token is the personal access token for HTTPS authentication.
	// Env: DAGU_GITSYNC_AUTH_TOKEN
	Token string `mapstructure:"token"`

	// SSHKeyPath is the path to the SSH private key file.
	// Env: DAGU_GITSYNC_AUTH_SSH_KEY_PATH
	SSHKeyPath string `mapstructure:"sshKeyPath"`

	// SSHPassphrase is the passphrase for the SSH key (optional).
	// Env: DAGU_GITSYNC_AUTH_SSH_PASSPHRASE
	SSHPassphrase string `mapstructure:"sshPassphrase"`
}

// GitSyncAutoSyncDef holds configuration for automatic synchronization.
type GitSyncAutoSyncDef struct {
	// Enabled indicates whether auto-sync is enabled.
	// Default: false
	// Env: DAGU_GITSYNC_AUTOSYNC_ENABLED
	Enabled *bool `mapstructure:"enabled"`

	// OnStartup indicates whether to sync on server startup.
	// Default: true
	// Env: DAGU_GITSYNC_AUTOSYNC_ON_STARTUP
	OnStartup *bool `mapstructure:"onStartup"`

	// Interval is the sync interval in seconds.
	// 0 means auto-sync is disabled (pull on startup only).
	// Default: 300 (5 minutes)
	// Env: DAGU_GITSYNC_AUTOSYNC_INTERVAL
	Interval int `mapstructure:"interval"`
}

// GitSyncCommitDef holds configuration for Git commits.
type GitSyncCommitDef struct {
	// AuthorName is the name to use for commits.
	// Default: Dagu
	// Env: DAGU_GITSYNC_COMMIT_AUTHOR_NAME
	AuthorName string `mapstructure:"authorName"`

	// AuthorEmail is the email to use for commits.
	// Default: dagu@localhost
	// Env: DAGU_GITSYNC_COMMIT_AUTHOR_EMAIL
	AuthorEmail string `mapstructure:"authorEmail"`
}

// TunnelDef holds the definition for tunnel configuration.
type TunnelDef struct {
	// Enabled indicates whether tunneling is enabled.
	// Default: false
	// Env: DAGU_TUNNEL_ENABLED
	Enabled *bool `mapstructure:"enabled"`

	// Provider specifies which tunnel provider to use: "cloudflare" or "tailscale".
	// Env: DAGU_TUNNEL_PROVIDER
	Provider string `mapstructure:"provider"`

	// Cloudflare contains Cloudflare Tunnel configuration.
	Cloudflare *CloudflareTunnelDef `mapstructure:"cloudflare"`

	// Tailscale contains Tailscale configuration.
	Tailscale *TailscaleTunnelDef `mapstructure:"tailscale"`

	// AllowTerminal allows terminal access via tunnel (default: false for security).
	// Env: DAGU_TUNNEL_ALLOW_TERMINAL
	AllowTerminal *bool `mapstructure:"allowTerminal"`

	// AllowedIPs is an IP allowlist (empty = allow all).
	// Env: DAGU_TUNNEL_ALLOWED_IPS
	AllowedIPs []string `mapstructure:"allowedIPs"`

	// RateLimiting contains rate limiting configuration for auth endpoints.
	RateLimiting *TunnelRateLimitDef `mapstructure:"rateLimiting"`
}

// CloudflareTunnelDef holds Cloudflare Tunnel settings.
type CloudflareTunnelDef struct {
	// Token is the Cloudflare Tunnel token (required for named tunnels).
	// Get this from Cloudflare Dashboard → Zero Trust → Tunnels.
	// Env: DAGU_TUNNEL_CLOUDFLARE_TOKEN
	Token string `mapstructure:"token"`

	// Hostname is the custom hostname for the tunnel.
	// If empty, uses the default cfargotunnel.com subdomain.
	// Env: DAGU_TUNNEL_CLOUDFLARE_HOSTNAME
	Hostname string `mapstructure:"hostname"`
}

// TailscaleTunnelDef holds Tailscale settings.
type TailscaleTunnelDef struct {
	// AuthKey is the Tailscale auth key for headless authentication.
	// If empty, interactive login via URL will be required.
	// Env: DAGU_TUNNEL_TAILSCALE_AUTH_KEY
	AuthKey string `mapstructure:"authKey"`

	// Hostname is the machine name in the tailnet (default: "dagu").
	// Env: DAGU_TUNNEL_TAILSCALE_HOSTNAME
	Hostname string `mapstructure:"hostname"`

	// Funnel enables Tailscale Funnel for public internet access.
	// When false, the server is only accessible within the tailnet.
	// Env: DAGU_TUNNEL_TAILSCALE_FUNNEL
	Funnel *bool `mapstructure:"funnel"`

	// StateDir is the directory for Tailscale state storage.
	// Default: $DAGU_HOME/tailscale
	// Env: DAGU_TUNNEL_TAILSCALE_STATE_DIR
	StateDir string `mapstructure:"stateDir"`
}

// TunnelRateLimitDef holds rate limiting configuration.
type TunnelRateLimitDef struct {
	// Enabled indicates whether rate limiting is enabled.
	// Env: DAGU_TUNNEL_RATE_LIMITING_ENABLED
	Enabled *bool `mapstructure:"enabled"`

	// LoginAttempts is the maximum login attempts per window.
	// Default: 5
	// Env: DAGU_TUNNEL_RATE_LIMITING_LOGIN_ATTEMPTS
	LoginAttempts int `mapstructure:"loginAttempts"`

	// WindowSeconds is the time window in seconds.
	// Default: 300 (5 minutes)
	// Env: DAGU_TUNNEL_RATE_LIMITING_WINDOW_SECONDS
	WindowSeconds int `mapstructure:"windowSeconds"`

	// BlockDurationSeconds is the block duration after exceeding limit.
	// Default: 900 (15 minutes)
	// Env: DAGU_TUNNEL_RATE_LIMITING_BLOCK_DURATION_SECONDS
	BlockDurationSeconds int `mapstructure:"blockDurationSeconds"`
}
